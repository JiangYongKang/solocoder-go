package schemamig

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrDuplicateVersion  = errors.New("schemamig: duplicate migration version")
	ErrVersionGap        = errors.New("schemamig: version number gap detected")
	ErrMigrationNotFound = errors.New("schemamig: migration not found")
	ErrLockAcquireFailed = errors.New("schemamig: failed to acquire migration lock")
	ErrLockTimeout       = errors.New("schemamig: migration lock acquire timeout")
	ErrAlreadyApplied    = errors.New("schemamig: migration already applied")
	ErrRollbackTarget    = errors.New("schemamig: rollback target version is higher than current version")
	ErrNoMigrations      = errors.New("schemamig: no migrations registered")
	ErrNotApplied        = errors.New("schemamig: migration not applied, cannot rollback")
	ErrEmptyUpSQL        = errors.New("schemamig: up SQL cannot be empty")
	ErrInvalidVersion    = errors.New("schemamig: version must be positive")
)

type Migration struct {
	Version     int
	Description string
	UpSQL       string
	DownSQL     string
}

type MigrationStatus struct {
	Version     int
	Description string
	Applied     bool
	AppliedAt   time.Time
}

type SQLExecutor interface {
	Exec(query string, args ...interface{}) error
	Query(query string, args ...interface{}) (Rows, error)
}

type Rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close() error
}

type Registry struct {
	mu         sync.RWMutex
	migrations map[int]*Migration
	versions   []int
}

func NewRegistry() *Registry {
	return &Registry{
		migrations: make(map[int]*Migration),
	}
}

func (r *Registry) Register(m *Migration) error {
	if m.Version <= 0 {
		return ErrInvalidVersion
	}
	if m.UpSQL == "" {
		return ErrEmptyUpSQL
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.migrations[m.Version]; exists {
		return fmt.Errorf("%w: version %d", ErrDuplicateVersion, m.Version)
	}

	r.migrations[m.Version] = m
	r.versions = append(r.versions, m.Version)
	sort.Ints(r.versions)
	return nil
}

func (r *Registry) MustRegister(m *Migration) {
	if err := r.Register(m); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(version int) (*Migration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.migrations[version]
	return m, ok
}

func (r *Registry) Versions() []int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]int, len(r.versions))
	copy(result, r.versions)
	return result
}

func (r *Registry) All() []*Migration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Migration, 0, len(r.versions))
	for _, v := range r.versions {
		m := *r.migrations[v]
		result = append(result, &m)
	}
	return result
}

func (r *Registry) Validate() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.versions) == 0 {
		return nil
	}

	if r.versions[0] != 1 {
		return fmt.Errorf("%w: first version is %d, expected 1", ErrVersionGap, r.versions[0])
	}

	for i := 1; i < len(r.versions); i++ {
		if r.versions[i] != r.versions[i-1]+1 {
			return fmt.Errorf("%w: gap between version %d and %d", ErrVersionGap, r.versions[i-1], r.versions[i])
		}
	}

	return nil
}

func (r *Registry) Range(from, to int) []*Migration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Migration
	for _, v := range r.versions {
		if v >= from && v <= to {
			result = append(result, r.migrations[v])
		}
	}
	return result
}

const (
	DefaultMigrationsTable = "schema_migrations"
	DefaultLockTable       = "schema_migration_lock"
	DefaultLockTimeout     = 30 * time.Second
)

type Migrator struct {
	registry  *Registry
	executor  SQLExecutor
	lock      *MigrationLock
	tableName string
	mu        sync.Mutex
}

func NewMigrator(registry *Registry, executor SQLExecutor, opts ...MigratorOption) (*Migrator, error) {
	if registry == nil {
		return nil, errors.New("schemamig: registry is required")
	}
	if executor == nil {
		return nil, errors.New("schemamig: executor is required")
	}

	cfg := &migratorConfig{
		tableName:  DefaultMigrationsTable,
		lockTable:  DefaultLockTable,
		lockTimeout: DefaultLockTimeout,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	lock := &MigrationLock{
		executor:  executor,
		lockTable: cfg.lockTable,
		timeout:   cfg.lockTimeout,
	}

	return &Migrator{
		registry:  registry,
		executor:  executor,
		lock:      lock,
		tableName: cfg.tableName,
	}, nil
}

type migratorConfig struct {
	tableName   string
	lockTable   string
	lockTimeout time.Duration
}

type MigratorOption func(*migratorConfig)

func WithTableName(name string) MigratorOption {
	return func(c *migratorConfig) { c.tableName = name }
}

func WithLockTable(name string) MigratorOption {
	return func(c *migratorConfig) { c.lockTable = name }
}

func WithLockTimeout(d time.Duration) MigratorOption {
	return func(c *migratorConfig) { c.lockTimeout = d }
}

func (m *Migrator) ensureMigrationsTable() error {
	sql := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (version INTEGER NOT NULL PRIMARY KEY, applied_at TIMESTAMP NOT NULL)`,
		m.tableName,
	)
	return m.executor.Exec(sql)
}

func (m *Migrator) ensureLockTable() error {
	sql := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (id INTEGER NOT NULL PRIMARY KEY, locked_at TIMESTAMP NOT NULL, locked_by VARCHAR(255) NOT NULL)`,
		m.lock.lockTable,
	)
	return m.executor.Exec(sql)
}

func (m *Migrator) getAppliedVersions() (map[int]time.Time, error) {
	sql := fmt.Sprintf(`SELECT version, applied_at FROM %s ORDER BY version`, m.tableName)
	rows, err := m.executor.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]time.Time)
	for rows.Next() {
		var version int
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return nil, err
		}
		applied[version] = appliedAt
	}
	return applied, nil
}

func (m *Migrator) recordApplied(version int) error {
	sql := fmt.Sprintf(`INSERT INTO %s (version, applied_at) VALUES (?, ?)`, m.tableName)
	return m.executor.Exec(sql, version, time.Now().UTC())
}

func (m *Migrator) removeApplied(version int) error {
	sql := fmt.Sprintf(`DELETE FROM %s WHERE version = ?`, m.tableName)
	return m.executor.Exec(sql, version)
}

func (m *Migrator) CurrentVersion() (int, error) {
	if err := m.ensureMigrationsTable(); err != nil {
		return 0, err
	}
	applied, err := m.getAppliedVersions()
	if err != nil {
		return 0, err
	}
	if len(applied) == 0 {
		return 0, nil
	}
	maxVer := 0
	for v := range applied {
		if v > maxVer {
			maxVer = v
		}
	}
	return maxVer, nil
}

func (m *Migrator) Status() ([]*MigrationStatus, error) {
	if err := m.ensureMigrationsTable(); err != nil {
		return nil, err
	}
	applied, err := m.getAppliedVersions()
	if err != nil {
		return nil, err
	}

	allMigrations := m.registry.All()
	statuses := make([]*MigrationStatus, 0, len(allMigrations))
	for _, mig := range allMigrations {
		s := &MigrationStatus{
			Version:     mig.Version,
			Description: mig.Description,
			Applied:     false,
		}
		if t, ok := applied[mig.Version]; ok {
			s.Applied = true
			s.AppliedAt = t
		}
		statuses = append(statuses, s)
	}
	return statuses, nil
}

func (m *Migrator) Up() ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureMigrationsTable(); err != nil {
		return nil, err
	}
	if err := m.ensureLockTable(); err != nil {
		return nil, err
	}

	if err := m.lock.Acquire(); err != nil {
		return nil, err
	}
	defer m.lock.Release()

	return m.upInternal()
}

func (m *Migrator) upInternal() ([]int, error) {
	applied, err := m.getAppliedVersions()
	if err != nil {
		return nil, err
	}

	allMigrations := m.registry.All()
	if len(allMigrations) == 0 {
		return nil, ErrNoMigrations
	}

	var appliedVersions []int
	for _, mig := range allMigrations {
		if _, ok := applied[mig.Version]; ok {
			continue
		}
		if err := m.executor.Exec(mig.UpSQL); err != nil {
			return appliedVersions, fmt.Errorf("schemamig: migration %d up failed: %w", mig.Version, err)
		}
		if err := m.recordApplied(mig.Version); err != nil {
			return appliedVersions, fmt.Errorf("schemamig: failed to record migration %d: %w", mig.Version, err)
		}
		appliedVersions = append(appliedVersions, mig.Version)
	}
	return appliedVersions, nil
}

func (m *Migrator) UpTo(targetVersion int) ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureMigrationsTable(); err != nil {
		return nil, err
	}
	if err := m.ensureLockTable(); err != nil {
		return nil, err
	}

	if err := m.lock.Acquire(); err != nil {
		return nil, err
	}
	defer m.lock.Release()

	applied, err := m.getAppliedVersions()
	if err != nil {
		return nil, err
	}

	allMigrations := m.registry.All()
	var appliedVersions []int
	for _, mig := range allMigrations {
		if mig.Version > targetVersion {
			break
		}
		if _, ok := applied[mig.Version]; ok {
			continue
		}
		if err := m.executor.Exec(mig.UpSQL); err != nil {
			return appliedVersions, fmt.Errorf("schemamig: migration %d up failed: %w", mig.Version, err)
		}
		if err := m.recordApplied(mig.Version); err != nil {
			return appliedVersions, fmt.Errorf("schemamig: failed to record migration %d: %w", mig.Version, err)
		}
		appliedVersions = append(appliedVersions, mig.Version)
	}
	return appliedVersions, nil
}

func (m *Migrator) Rollback(targetVersion int) ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureMigrationsTable(); err != nil {
		return nil, err
	}
	if err := m.ensureLockTable(); err != nil {
		return nil, err
	}

	if err := m.lock.Acquire(); err != nil {
		return nil, err
	}
	defer m.lock.Release()

	return m.rollbackInternal(targetVersion)
}

func (m *Migrator) rollbackInternal(targetVersion int) ([]int, error) {
	current, err := m.CurrentVersion()
	if err != nil {
		return nil, err
	}
	if targetVersion > current {
		return nil, fmt.Errorf("%w: target %d > current %d", ErrRollbackTarget, targetVersion, current)
	}
	if targetVersion == current {
		return nil, nil
	}

	allMigrations := m.registry.All()
	var rolledBack []int
	for i := len(allMigrations) - 1; i >= 0; i-- {
		mig := allMigrations[i]
		if mig.Version <= targetVersion {
			break
		}
		if mig.DownSQL == "" {
			return rolledBack, fmt.Errorf("schemamig: migration %d has no down SQL", mig.Version)
		}
		if err := m.executor.Exec(mig.DownSQL); err != nil {
			return rolledBack, fmt.Errorf("schemamig: migration %d down failed: %w", mig.Version, err)
		}
		if err := m.removeApplied(mig.Version); err != nil {
			return rolledBack, fmt.Errorf("schemamig: failed to remove migration %d record: %w", mig.Version, err)
		}
		rolledBack = append(rolledBack, mig.Version)
	}
	return rolledBack, nil
}

type MigrationLock struct {
	executor  SQLExecutor
	lockTable string
	timeout   time.Duration
	mu        sync.Mutex
	locked    bool
	lockedAt  time.Time
	lockedBy  string
}

func (l *MigrationLock) Acquire() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.locked {
		return fmt.Errorf("%w: already locked by %s", ErrLockAcquireFailed, l.lockedBy)
	}

	if err := l.executor.Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE locked_at < ?`, l.lockTable),
		time.Now().Add(-l.timeout).UTC(),
	); err != nil {
		return fmt.Errorf("%w: cleanup expired locks failed: %v", ErrLockAcquireFailed, err)
	}

	lockID := 1
	lockedBy := fmt.Sprintf("pid_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
	lockedAt := time.Now().UTC()

	if err := l.executor.Exec(
		fmt.Sprintf(`INSERT INTO %s (id, locked_at, locked_by) VALUES (?, ?, ?)`, l.lockTable),
		lockID, lockedAt, lockedBy,
	); err != nil {
		return fmt.Errorf("%w: %v", ErrLockAcquireFailed, err)
	}

	l.locked = true
	l.lockedAt = lockedAt
	l.lockedBy = lockedBy
	return nil
}

func (l *MigrationLock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.locked {
		return nil
	}

	if err := l.executor.Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE id = 1`, l.lockTable),
	); err != nil {
		return fmt.Errorf("schemamig: failed to release lock: %w", err)
	}

	l.locked = false
	l.lockedAt = time.Time{}
	l.lockedBy = ""
	return nil
}

func (l *MigrationLock) IsLocked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.locked
}

func (l *MigrationLock) TryAcquire(timeout time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.locked {
		return fmt.Errorf("%w: already locked", ErrLockAcquireFailed)
	}

	deadline := time.Now().Add(timeout)
	for {
		if err := l.executor.Exec(
			fmt.Sprintf(`DELETE FROM %s WHERE locked_at < ?`, l.lockTable),
			time.Now().Add(-l.timeout).UTC(),
		); err != nil {
			return fmt.Errorf("%w: cleanup expired locks failed: %v", ErrLockAcquireFailed, err)
		}

		lockID := 1
		lockedBy := fmt.Sprintf("pid_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
		lockedAt := time.Now().UTC()

		if err := l.executor.Exec(
			fmt.Sprintf(`INSERT INTO %s (id, locked_at, locked_by) VALUES (?, ?, ?)`, l.lockTable),
			lockID, lockedAt, lockedBy,
		); err == nil {
			l.locked = true
			l.lockedAt = lockedAt
			l.lockedBy = lockedBy
			return nil
		}

		if time.Now().After(deadline) {
			return ErrLockTimeout
		}
		time.Sleep(50 * time.Millisecond)
	}
}
