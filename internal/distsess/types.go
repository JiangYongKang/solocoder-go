package distsess

import (
	"errors"
	"time"
)

var (
	ErrSessionNotFound    = errors.New("distsess: session not found")
	ErrSessionExpired     = errors.New("distsess: session expired")
	ErrInvalidSessionID   = errors.New("distsess: invalid session id")
	ErrInvalidConfig      = errors.New("distsess: invalid configuration")
	ErrPersistenceFailed  = errors.New("distsess: persistence operation failed")
	ErrNodeNotFound       = errors.New("distsess: node not found")
	ErrClusterStopped     = errors.New("distsess: cluster is stopped")
	ErrInvalidMessageType = errors.New("distsess: invalid message type")
	ErrVersionTooOld      = errors.New("distsess: update rejected due to older version")
	ErrMigrationFailed    = errors.New("distsess: migration operation failed")
	ErrChecksumMismatch   = errors.New("distsess: checksum mismatch during migration")
	ErrSessionExists      = errors.New("distsess: session already exists")
	ErrEmptySessionID     = errors.New("distsess: session id cannot be empty")
	ErrNilSessionData     = errors.New("distsess: session data cannot be nil")
)

const (
	DefaultTTL             = 30 * time.Minute
	DefaultCleanupInterval = 1 * time.Minute
	DefaultSyncBuffer      = 1024
	DefaultAutoRenew       = true
	MigrationFormatVersion = 1
)

type SessionData map[string]interface{}

type Session struct {
	ID        string
	Data      SessionData
	TTL       time.Duration
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   uint64
	NodeID    string
}

func (s *Session) IsExpired() bool {
	if s.TTL <= 0 {
		return false
	}
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) Renew() {
	if s.TTL <= 0 {
		return
	}
	s.ExpiresAt = time.Now().Add(s.TTL)
	s.UpdatedAt = time.Now()
	s.Version++
}

func (s *Session) DeepCopy() *Session {
	data := make(SessionData, len(s.Data))
	for k, v := range s.Data {
		data[k] = v
	}
	return &Session{
		ID:        s.ID,
		Data:      data,
		TTL:       s.TTL,
		ExpiresAt: s.ExpiresAt,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		Version:   s.Version,
		NodeID:    s.NodeID,
	}
}

type ChangeType int

const (
	ChangeTypeCreate ChangeType = iota
	ChangeTypeUpdate
	ChangeTypeDelete
	ChangeTypeRenew
)

type ChangeNotification struct {
	Type       ChangeType
	SessionID  string
	Version    uint64
	NodeID     string
	Timestamp  time.Time
	DataDigest string
	Data       *Session
}

type MessageType int

const (
	MsgChangeNotify MessageType = iota
	MsgSyncRequest
	MsgSyncResponse
	MsgInvalidate
)

type Message struct {
	Type       MessageType
	FromNodeID string
	ToNodeID   string
	SessionID  string
	Session    *Session
	Version    uint64
	Timestamp  time.Time
	Digest     string
	Sessions   []*Session
}

type Config struct {
	NodeID          string
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
	AutoRenew       bool
	PersistenceDir  string
	SyncBuffer      int
	EnableSync      bool
}

func DefaultConfig() Config {
	return Config{
		NodeID:          generateNodeID(),
		DefaultTTL:      DefaultTTL,
		CleanupInterval: DefaultCleanupInterval,
		AutoRenew:       DefaultAutoRenew,
		PersistenceDir:  "distsess_data",
		SyncBuffer:      DefaultSyncBuffer,
		EnableSync:      true,
	}
}

func generateNodeID() string {
	return "node-" + time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(1 * time.Nanosecond)
	}
	return string(b)
}

type Stats struct {
	MemoryCount   int
	PersistedCount int
	ExpiredCount  uint64
	SyncedCount   uint64
	HitCount      uint64
	MissCount     uint64
}
