package distsess

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type PersistenceStore interface {
	Save(session *Session) error
	Load(sessionID string) (*Session, error)
	Delete(sessionID string) error
	LoadAll() ([]*Session, error)
	Count() (int, error)
	Clear() error
	Close() error
}

type FilePersistenceStore struct {
	dir string
	mu  sync.RWMutex
}

func NewFilePersistenceStore(dir string) (*FilePersistenceStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("%w: persistence directory cannot be empty", ErrInvalidConfig)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("%w: failed to create persistence directory: %v", ErrPersistenceFailed, err)
	}

	return &FilePersistenceStore{
		dir: dir,
	}, nil
}

func (fps *FilePersistenceStore) sessionFilename(sessionID string) string {
	hash := sha256.Sum256([]byte(sessionID))
	return filepath.Join(fps.dir, hex.EncodeToString(hash[:])+".json")
}

func (fps *FilePersistenceStore) Save(session *Session) error {
	if session == nil {
		return ErrNilSessionData
	}
	if session.ID == "" {
		return ErrEmptySessionID
	}

	fps.mu.Lock()
	defer fps.mu.Unlock()

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal session: %v", ErrPersistenceFailed, err)
	}

	filename := fps.sessionFilename(session.ID)
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("%w: failed to write session file: %v", ErrPersistenceFailed, err)
	}

	return nil
}

func (fps *FilePersistenceStore) Load(sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, ErrEmptySessionID
	}

	fps.mu.RLock()
	defer fps.mu.RUnlock()

	filename := fps.sessionFilename(sessionID)
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("%w: failed to read session file: %v", ErrPersistenceFailed, err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal session: %v", ErrPersistenceFailed, err)
	}

	return &session, nil
}

func (fps *FilePersistenceStore) Delete(sessionID string) error {
	if sessionID == "" {
		return ErrEmptySessionID
	}

	fps.mu.Lock()
	defer fps.mu.Unlock()

	filename := fps.sessionFilename(sessionID)
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%w: failed to delete session file: %v", ErrPersistenceFailed, err)
	}

	return nil
}

func (fps *FilePersistenceStore) LoadAll() ([]*Session, error) {
	fps.mu.RLock()
	defer fps.mu.RUnlock()

	files, err := os.ReadDir(fps.dir)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read persistence directory: %v", ErrPersistenceFailed, err)
	}

	sessions := make([]*Session, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filename := filepath.Join(fps.dir, file.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (fps *FilePersistenceStore) Count() (int, error) {
	fps.mu.RLock()
	defer fps.mu.RUnlock()

	files, err := os.ReadDir(fps.dir)
	if err != nil {
		return 0, fmt.Errorf("%w: failed to read persistence directory: %v", ErrPersistenceFailed, err)
	}

	count := 0
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			count++
		}
	}

	return count, nil
}

func (fps *FilePersistenceStore) Clear() error {
	fps.mu.Lock()
	defer fps.mu.Unlock()

	files, err := os.ReadDir(fps.dir)
	if err != nil {
		return fmt.Errorf("%w: failed to read persistence directory: %v", ErrPersistenceFailed, err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		filename := filepath.Join(fps.dir, file.Name())
		os.Remove(filename)
	}

	return nil
}

func (fps *FilePersistenceStore) Close() error {
	return nil
}

func computeDataDigest(data SessionData) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(":"))
		if v, ok := data[k].(string); ok {
			h.Write([]byte(v))
		} else {
			h.Write([]byte(fmt.Sprintf("%v", data[k])))
		}
		h.Write([]byte(";"))
	}

	return hex.EncodeToString(h.Sum(nil))
}

type MemoryPersistenceStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewMemoryPersistenceStore() *MemoryPersistenceStore {
	return &MemoryPersistenceStore{
		sessions: make(map[string]*Session),
	}
}

func (mps *MemoryPersistenceStore) Save(session *Session) error {
	if session == nil {
		return ErrNilSessionData
	}
	if session.ID == "" {
		return ErrEmptySessionID
	}

	mps.mu.Lock()
	defer mps.mu.Unlock()

	mps.sessions[session.ID] = session.DeepCopy()
	return nil
}

func (mps *MemoryPersistenceStore) Load(sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, ErrEmptySessionID
	}

	mps.mu.RLock()
	defer mps.mu.RUnlock()

	session, ok := mps.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}

	return session.DeepCopy(), nil
}

func (mps *MemoryPersistenceStore) Delete(sessionID string) error {
	if sessionID == "" {
		return ErrEmptySessionID
	}

	mps.mu.Lock()
	defer mps.mu.Unlock()

	delete(mps.sessions, sessionID)
	return nil
}

func (mps *MemoryPersistenceStore) LoadAll() ([]*Session, error) {
	mps.mu.RLock()
	defer mps.mu.RUnlock()

	sessions := make([]*Session, 0, len(mps.sessions))
	for _, s := range mps.sessions {
		sessions = append(sessions, s.DeepCopy())
	}

	return sessions, nil
}

func (mps *MemoryPersistenceStore) Count() (int, error) {
	mps.mu.RLock()
	defer mps.mu.RUnlock()

	return len(mps.sessions), nil
}

func (mps *MemoryPersistenceStore) Clear() error {
	mps.mu.Lock()
	defer mps.mu.Unlock()

	mps.sessions = make(map[string]*Session)
	return nil
}

func (mps *MemoryPersistenceStore) Close() error {
	return nil
}
