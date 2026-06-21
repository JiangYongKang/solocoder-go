package distsess

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type MigrationFormat string

const (
	MigrationFormatJSON MigrationFormat = "json"
)

type MigrationHeader struct {
	FormatVersion  int             `json:"format_version"`
	ExportedAt     time.Time       `json:"exported_at"`
	SourceNodeID   string          `json:"source_node_id,omitempty"`
	SessionCount   int             `json:"session_count"`
	Checksum       string          `json:"checksum"`
	Format         MigrationFormat `json:"format"`
}

type MigrationData struct {
	Header   MigrationHeader `json:"header"`
	Sessions []*Session      `json:"sessions"`
}

type MigrationResult struct {
	ImportedCount int
	SkippedCount  int
	FailedCount   int
	Errors        []error
}

func ExportSession(session *Session) ([]byte, error) {
	if session == nil {
		return nil, ErrNilSessionData
	}
	if session.ID == "" {
		return nil, ErrEmptySessionID
	}

	sessions := []*Session{session.DeepCopy()}
	return exportSessions(sessions, "")
}

func ExportAllSessions(node *Node) ([]byte, error) {
	if node == nil {
		return nil, fmt.Errorf("%w: node cannot be nil", ErrMigrationFailed)
	}

	sessions, err := node.GetAll()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get sessions: %v", ErrMigrationFailed, err)
	}

	return exportSessions(sessions, node.ID)
}

func exportSessions(sessions []*Session, sourceNodeID string) ([]byte, error) {
	sortedSessions := make([]*Session, len(sessions))
	copy(sortedSessions, sessions)
	sort.Slice(sortedSessions, func(i, j int) bool {
		return sortedSessions[i].ID < sortedSessions[j].ID
	})

	checksum := computeChecksum(sortedSessions)

	header := MigrationHeader{
		FormatVersion: MigrationFormatVersion,
		ExportedAt:    time.Now(),
		SourceNodeID:  sourceNodeID,
		SessionCount:  len(sortedSessions),
		Checksum:      checksum,
		Format:        MigrationFormatJSON,
	}

	migrationData := MigrationData{
		Header:   header,
		Sessions: sortedSessions,
	}

	data, err := json.MarshalIndent(migrationData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w: failed to marshal migration data: %v", ErrMigrationFailed, err)
	}

	return data, nil
}

func ImportSession(data []byte, node *Node) (*MigrationResult, error) {
	return importSessions(data, node, false)
}

func ImportAllSessions(data []byte, node *Node, overwrite bool) (*MigrationResult, error) {
	return importSessions(data, node, overwrite)
}

func importSessions(data []byte, node *Node, overwrite bool) (*MigrationResult, error) {
	if node == nil {
		return nil, fmt.Errorf("%w: node cannot be nil", ErrMigrationFailed)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty data", ErrMigrationFailed)
	}

	var migrationData MigrationData
	if err := json.Unmarshal(data, &migrationData); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal migration data: %v", ErrMigrationFailed, err)
	}

	if migrationData.Header.FormatVersion != MigrationFormatVersion {
		return nil, fmt.Errorf("%w: unsupported format version %d, expected %d",
			ErrMigrationFailed, migrationData.Header.FormatVersion, MigrationFormatVersion)
	}

	sortedSessions := make([]*Session, len(migrationData.Sessions))
	copy(sortedSessions, migrationData.Sessions)
	sort.Slice(sortedSessions, func(i, j int) bool {
		return sortedSessions[i].ID < sortedSessions[j].ID
	})

	actualChecksum := computeChecksum(sortedSessions)
	if actualChecksum != migrationData.Header.Checksum {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, migrationData.Header.Checksum, actualChecksum)
	}

	result := &MigrationResult{
		Errors: make([]error, 0),
	}

	now := time.Now()
	for _, session := range sortedSessions {
		if session == nil || session.ID == "" {
			result.SkippedCount++
			result.Errors = append(result.Errors, fmt.Errorf("invalid session: %v", session))
			continue
		}

		if session.TTL > 0 && now.After(session.ExpiresAt) {
			result.SkippedCount++
			continue
		}

		if !overwrite && node.Exists(session.ID) {
			result.SkippedCount++
			continue
		}

		sessionCopy := session.DeepCopy()
		_, err := node.store.SetWithTTL(sessionCopy.ID, sessionCopy.Data, sessionCopy.TTL)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Errorf("session %s: %w", sessionCopy.ID, err))
			continue
		}

		result.ImportedCount++
	}

	return result, nil
}

func computeChecksum(sessions []*Session) string {
	h := sha256.New()

	for _, s := range sessions {
		if s == nil {
			continue
		}

		h.Write([]byte(s.ID))
		h.Write([]byte("|"))

		keys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			h.Write([]byte(k))
			h.Write([]byte(":"))
			if v, ok := s.Data[k].(string); ok {
				h.Write([]byte(v))
			} else {
				h.Write([]byte(fmt.Sprintf("%v", s.Data[k])))
			}
			h.Write([]byte(";"))
		}

		h.Write([]byte(fmt.Sprintf("|%d|%d", s.Version, s.TTL.Nanoseconds())))
		h.Write([]byte("\n"))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func ValidateMigrationData(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("%w: empty data", ErrMigrationFailed)
	}

	var migrationData MigrationData
	if err := json.Unmarshal(data, &migrationData); err != nil {
		return fmt.Errorf("%w: failed to unmarshal: %v", ErrMigrationFailed, err)
	}

	if migrationData.Header.FormatVersion != MigrationFormatVersion {
		return fmt.Errorf("%w: unsupported format version %d", ErrMigrationFailed, migrationData.Header.FormatVersion)
	}

	if migrationData.Header.SessionCount != len(migrationData.Sessions) {
		return fmt.Errorf("%w: session count mismatch: header=%d, actual=%d",
			ErrMigrationFailed, migrationData.Header.SessionCount, len(migrationData.Sessions))
	}

	sortedSessions := make([]*Session, len(migrationData.Sessions))
	copy(sortedSessions, migrationData.Sessions)
	sort.Slice(sortedSessions, func(i, j int) bool {
		return sortedSessions[i].ID < sortedSessions[j].ID
	})

	actualChecksum := computeChecksum(sortedSessions)
	if actualChecksum != migrationData.Header.Checksum {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, migrationData.Header.Checksum, actualChecksum)
	}

	for _, s := range migrationData.Sessions {
		if s == nil {
			return fmt.Errorf("%w: nil session in data", ErrMigrationFailed)
		}
		if s.ID == "" {
			return fmt.Errorf("%w: session with empty ID", ErrMigrationFailed)
		}
	}

	return nil
}
