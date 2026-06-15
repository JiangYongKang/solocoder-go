package schemamig

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type mockRow struct {
	values []interface{}
}

type mockRows struct {
	data   [][]interface{}
	idx    int
	closed bool
}

func (r *mockRows) Next() bool {
	r.idx++
	return r.idx <= len(r.data)
}

func (r *mockRows) Scan(dest ...interface{}) error {
	if r.idx < 1 || r.idx > len(r.data) {
		return fmt.Errorf("no more rows")
	}
	row := r.data[r.idx-1]
	for i := range dest {
		if i >= len(row) {
			return fmt.Errorf("not enough columns")
		}
		switch d := dest[i].(type) {
		case *int:
			val, ok := row[i].(int)
			if !ok {
				return fmt.Errorf("type mismatch at column %d", i)
			}
			*d = val
		case *time.Time:
			val, ok := row[i].(time.Time)
			if !ok {
				return fmt.Errorf("type mismatch at column %d", i)
			}
			*d = val
		case *string:
			val, ok := row[i].(string)
			if !ok {
				return fmt.Errorf("type mismatch at column %d", i)
			}
			*d = val
		default:
			return fmt.Errorf("unsupported scan type at column %d", i)
		}
	}
	return nil
}

func (r *mockRows) Close() error {
	r.closed = true
	return nil
}

type execRecord struct {
	query string
	args  []interface{}
}

type mockExecutor struct {
	mu          sync.Mutex
	execLog     []execRecord
	applied     map[int]time.Time
	lockExists  bool
	lockLockedBy string
	lockLockedAt time.Time
	execErr     map[string]error
	queryErr    map[string]error
	scanErr     bool
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		applied:  make(map[int]time.Time),
		execErr:  make(map[string]error),
		queryErr: make(map[string]error),
	}
}

func (e *mockExecutor) Exec(query string, args ...interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.execLog = append(e.execLog, execRecord{query: query, args: args})

	for pattern, err := range e.execErr {
		if containsPattern(query, pattern) {
			return err
		}
	}

	if containsPattern(query, "INSERT INTO schema_migrations") && len(args) >= 2 {
		version, ok := args[0].(int)
		if ok {
			appliedAt, ok := args[1].(time.Time)
			if ok {
				e.applied[version] = appliedAt
			}
		}
	}

	if containsPattern(query, "DELETE FROM schema_migrations") && len(args) >= 1 {
		version, ok := args[0].(int)
		if ok {
			delete(e.applied, version)
		}
	}

	if containsPattern(query, "INSERT INTO schema_migration_lock") {
		e.lockExists = true
		if len(args) >= 3 {
			if lockedBy, ok := args[2].(string); ok {
				e.lockLockedBy = lockedBy
			}
			if lockedAt, ok := args[1].(time.Time); ok {
				e.lockLockedAt = lockedAt
			}
		}
	}

	if containsPattern(query, "DELETE FROM schema_migration_lock") {
		e.lockExists = false
		e.lockLockedBy = ""
		e.lockLockedAt = time.Time{}
	}

	return nil
}

func (e *mockExecutor) Query(query string, args ...interface{}) (Rows, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for pattern, err := range e.queryErr {
		if containsPattern(query, pattern) {
			return nil, err
		}
	}

	if containsPattern(query, "SELECT version, applied_at FROM schema_migrations") {
		var data [][]interface{}
		for v, t := range e.applied {
			data = append(data, []interface{}{v, t})
		}
		return &mockRows{data: data}, nil
	}

	return &mockRows{data: nil}, nil
}

func containsPattern(s, pattern string) bool {
	return len(s) >= len(pattern) && (s == pattern || len(pattern) > 0 && containsStr(s, pattern))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func newTestRegistry() *Registry {
	r := NewRegistry()
	r.MustRegister(&Migration{Version: 1, Description: "create users table", UpSQL: "CREATE TABLE users (id INT)", DownSQL: "DROP TABLE users"})
	r.MustRegister(&Migration{Version: 2, Description: "add email column", UpSQL: "ALTER TABLE users ADD COLUMN email VARCHAR(255)", DownSQL: "ALTER TABLE users DROP COLUMN email"})
	r.MustRegister(&Migration{Version: 3, Description: "create posts table", UpSQL: "CREATE TABLE posts (id INT)", DownSQL: "DROP TABLE posts"})
	return r
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	m := &Migration{Version: 1, Description: "test", UpSQL: "CREATE TABLE t1 (id INT)", DownSQL: "DROP TABLE t1"}
	if err := r.Register(m); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if len(r.Versions()) != 1 {
		t.Errorf("Versions() length = %d, want 1", len(r.Versions()))
	}
	if r.Versions()[0] != 1 {
		t.Errorf("Versions()[0] = %d, want 1", r.Versions()[0])
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&Migration{Version: 1, Description: "first", UpSQL: "SQL1"})
	err := r.Register(&Migration{Version: 1, Description: "duplicate", UpSQL: "SQL2"})
	if !errors.Is(err, ErrDuplicateVersion) {
		t.Errorf("Register duplicate error = %v, want ErrDuplicateVersion", err)
	}
}

func TestRegistry_RegisterInvalidVersion(t *testing.T) {
	r := NewRegistry()
	err := r.Register(&Migration{Version: 0, Description: "zero", UpSQL: "SQL"})
	if !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("Register version 0 error = %v, want ErrInvalidVersion", err)
	}
	err = r.Register(&Migration{Version: -1, Description: "negative", UpSQL: "SQL"})
	if !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("Register version -1 error = %v, want ErrInvalidVersion", err)
	}
}

func TestRegistry_RegisterEmptyUpSQL(t *testing.T) {
	r := NewRegistry()
	err := r.Register(&Migration{Version: 1, Description: "test", UpSQL: ""})
	if !errors.Is(err, ErrEmptyUpSQL) {
		t.Errorf("Register empty UpSQL error = %v, want ErrEmptyUpSQL", err)
	}
}

func TestRegistry_MustRegister(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Migration{Version: 1, Description: "test", UpSQL: "SQL"})

	defer func() {
		if rec := recover(); rec == nil {
			t.Error("MustRegister with duplicate should panic")
		}
	}()
	r.MustRegister(&Migration{Version: 1, Description: "dup", UpSQL: "SQL2"})
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Migration{Version: 1, Description: "test", UpSQL: "SQL"})

	m, ok := r.Get(1)
	if !ok {
		t.Fatal("Get(1) should exist")
	}
	if m.Version != 1 {
		t.Errorf("Get(1).Version = %d, want 1", m.Version)
	}

	_, ok = r.Get(99)
	if ok {
		t.Error("Get(99) should not exist")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Migration{Version: 3, Description: "third", UpSQL: "SQL3"})
	r.MustRegister(&Migration{Version: 1, Description: "first", UpSQL: "SQL1"})
	r.MustRegister(&Migration{Version: 2, Description: "second", UpSQL: "SQL2"})

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("All() length = %d, want 3", len(all))
	}
	for i, m := range all {
		if m.Version != i+1 {
			t.Errorf("All()[%d].Version = %d, want %d", i, m.Version, i+1)
		}
	}
}

func TestRegistry_Validate(t *testing.T) {
	tests := []struct {
		name      string
		versions  []int
		wantErr   bool
		errTarget error
	}{
		{"empty", []int{}, false, nil},
		{"continuous from 1", []int{1, 2, 3}, false, nil},
		{"not starting from 1", []int{2, 3}, true, ErrVersionGap},
		{"gap in middle", []int{1, 3}, true, ErrVersionGap},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			for _, v := range tt.versions {
				r.MustRegister(&Migration{Version: v, Description: fmt.Sprintf("v%d", v), UpSQL: fmt.Sprintf("SQL%d", v)})
			}
			err := r.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Validate() should return error")
				}
				if tt.errTarget != nil && !errors.Is(err, tt.errTarget) {
					t.Errorf("Validate() error = %v, want %v", err, tt.errTarget)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
			}
		})
	}
}

func TestRegistry_Range(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Migration{Version: 1, Description: "v1", UpSQL: "SQL1"})
	r.MustRegister(&Migration{Version: 2, Description: "v2", UpSQL: "SQL2"})
	r.MustRegister(&Migration{Version: 3, Description: "v3", UpSQL: "SQL3"})
	r.MustRegister(&Migration{Version: 4, Description: "v4", UpSQL: "SQL4"})

	result := r.Range(2, 3)
	if len(result) != 2 {
		t.Fatalf("Range(2,3) length = %d, want 2", len(result))
	}
	if result[0].Version != 2 || result[1].Version != 3 {
		t.Errorf("Range(2,3) versions = [%d, %d], want [2, 3]", result[0].Version, result[1].Version)
	}
}

func TestNewMigrator_NilRegistry(t *testing.T) {
	_, err := NewMigrator(nil, newMockExecutor())
	if err == nil {
		t.Error("NewMigrator with nil registry should return error")
	}
}

func TestNewMigrator_NilExecutor(t *testing.T) {
	_, err := NewMigrator(NewRegistry(), nil)
	if err == nil {
		t.Error("NewMigrator with nil executor should return error")
	}
}

func TestNewMigrator_WithOptions(t *testing.T) {
	exec := newMockExecutor()
	m, err := NewMigrator(NewRegistry(), exec,
		WithTableName("custom_migrations"),
		WithLockTable("custom_lock"),
		WithLockTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("NewMigrator() error = %v", err)
	}
	if m.tableName != "custom_migrations" {
		t.Errorf("tableName = %q, want %q", m.tableName, "custom_migrations")
	}
	if m.lock.lockTable != "custom_lock" {
		t.Errorf("lockTable = %q, want %q", m.lock.lockTable, "custom_lock")
	}
	if m.lock.timeout != 10*time.Second {
		t.Errorf("lockTimeout = %v, want %v", m.lock.timeout, 10*time.Second)
	}
}

func TestMigrator_Up_AllPending(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, err := NewMigrator(reg, exec)
	if err != nil {
		t.Fatalf("NewMigrator() error = %v", err)
	}

	applied, err := m.Up()
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if len(applied) != 3 {
		t.Fatalf("Up() applied %d, want 3", len(applied))
	}
	for i, v := range []int{1, 2, 3} {
		if applied[i] != v {
			t.Errorf("applied[%d] = %d, want %d", i, applied[i], v)
		}
	}

	current, err := m.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion() error = %v", err)
	}
	if current != 3 {
		t.Errorf("CurrentVersion() = %d, want 3", current)
	}
}

func TestMigrator_Up_NoPending(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	applied, err := m.Up()
	if err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("second Up() applied %d, want 0", len(applied))
	}
}

func TestMigrator_Up_Idempotent(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, err := m.Up()
	if err != nil {
		t.Fatalf("first Up() error = %v", err)
	}

	applied, err := m.Up()
	if err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("second Up() should apply 0 migrations, got %d", len(applied))
	}

	current, _ := m.CurrentVersion()
	if current != 3 {
		t.Errorf("CurrentVersion() = %d, want 3", current)
	}
}

func TestMigrator_Up_NoMigrations(t *testing.T) {
	exec := newMockExecutor()
	reg := NewRegistry()
	m, _ := NewMigrator(reg, exec)

	_, err := m.Up()
	if !errors.Is(err, ErrNoMigrations) {
		t.Errorf("Up() with empty registry error = %v, want ErrNoMigrations", err)
	}
}

func TestMigrator_UpTo(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	applied, err := m.UpTo(2)
	if err != nil {
		t.Fatalf("UpTo(2) error = %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("UpTo(2) applied %d, want 2", len(applied))
	}
	if applied[0] != 1 || applied[1] != 2 {
		t.Errorf("UpTo(2) versions = %v, want [1, 2]", applied)
	}

	current, _ := m.CurrentVersion()
	if current != 2 {
		t.Errorf("CurrentVersion() = %d, want 2", current)
	}
}

func TestMigrator_UpTo_AlreadyApplied(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	applied, err := m.UpTo(2)
	if err != nil {
		t.Fatalf("UpTo(2) after full Up error = %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("UpTo(2) after full Up applied %d, want 0", len(applied))
	}
}

func TestMigrator_UpTo_PartialThenFull(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	applied1, err := m.UpTo(1)
	if err != nil {
		t.Fatalf("UpTo(1) error = %v", err)
	}
	if len(applied1) != 1 || applied1[0] != 1 {
		t.Errorf("UpTo(1) = %v, want [1]", applied1)
	}

	applied2, err := m.Up()
	if err != nil {
		t.Fatalf("Up() after UpTo(1) error = %v", err)
	}
	if len(applied2) != 2 {
		t.Fatalf("Up() after UpTo(1) applied %d, want 2", len(applied2))
	}
	if applied2[0] != 2 || applied2[1] != 3 {
		t.Errorf("Up() after UpTo(1) = %v, want [2, 3]", applied2)
	}
}

func TestMigrator_Rollback(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	rolledBack, err := m.Rollback(1)
	if err != nil {
		t.Fatalf("Rollback(1) error = %v", err)
	}
	if len(rolledBack) != 2 {
		t.Fatalf("Rollback(1) rolled back %d, want 2", len(rolledBack))
	}
	if rolledBack[0] != 3 || rolledBack[1] != 2 {
		t.Errorf("Rollback(1) versions = %v, want [3, 2]", rolledBack)
	}

	current, _ := m.CurrentVersion()
	if current != 1 {
		t.Errorf("CurrentVersion() after rollback = %d, want 1", current)
	}
}

func TestMigrator_Rollback_ToZero(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	rolledBack, err := m.Rollback(0)
	if err != nil {
		t.Fatalf("Rollback(0) error = %v", err)
	}
	if len(rolledBack) != 3 {
		t.Fatalf("Rollback(0) rolled back %d, want 3", len(rolledBack))
	}

	current, _ := m.CurrentVersion()
	if current != 0 {
		t.Errorf("CurrentVersion() after rollback to 0 = %d, want 0", current)
	}
}

func TestMigrator_Rollback_SameVersion(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	rolledBack, err := m.Rollback(3)
	if err != nil {
		t.Fatalf("Rollback(3) error = %v", err)
	}
	if len(rolledBack) != 0 {
		t.Errorf("Rollback(3) rolled back %d, want 0", len(rolledBack))
	}
}

func TestMigrator_Rollback_TargetHigherThanCurrent(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.UpTo(2)

	_, err := m.Rollback(3)
	if !errors.Is(err, ErrRollbackTarget) {
		t.Errorf("Rollback(3) when current=2 error = %v, want ErrRollbackTarget", err)
	}
}

func TestMigrator_Rollback_NoAppliedMigrations(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, err := m.Rollback(1)
	if !errors.Is(err, ErrRollbackTarget) {
		t.Errorf("Rollback(1) with no migrations error = %v, want ErrRollbackTarget", err)
	}
}

func TestMigrator_Rollback_NoDownSQL(t *testing.T) {
	exec := newMockExecutor()
	reg := NewRegistry()
	reg.MustRegister(&Migration{Version: 1, Description: "no down", UpSQL: "CREATE TABLE t (id INT)", DownSQL: ""})
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	_, err := m.Rollback(0)
	if err == nil {
		t.Error("Rollback with no down SQL should return error")
	}
}

func TestMigrator_CurrentVersion_Empty(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	current, err := m.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion() error = %v", err)
	}
	if current != 0 {
		t.Errorf("CurrentVersion() = %d, want 0", current)
	}
}

func TestMigrator_Status(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	statuses, err := m.Status()
	if err != nil {
		t.Fatalf("Status() before Up error = %v", err)
	}
	for _, s := range statuses {
		if s.Applied {
			t.Errorf("Status()[v%d].Applied = true, want false before Up", s.Version)
		}
	}

	_, _ = m.Up()

	statuses, err = m.Status()
	if err != nil {
		t.Fatalf("Status() after Up error = %v", err)
	}
	for _, s := range statuses {
		if !s.Applied {
			t.Errorf("Status()[v%d].Applied = false, want true after Up", s.Version)
		}
		if s.AppliedAt.IsZero() {
			t.Errorf("Status()[v%d].AppliedAt is zero", s.Version)
		}
	}
}

func TestMigrator_Status_PartialApplied(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.UpTo(2)

	statuses, _ := m.Status()
	if len(statuses) != 3 {
		t.Fatalf("Status() length = %d, want 3", len(statuses))
	}
	for _, s := range statuses {
		if s.Version <= 2 && !s.Applied {
			t.Errorf("Status()[v%d] should be applied", s.Version)
		}
		if s.Version == 3 && s.Applied {
			t.Errorf("Status()[v3] should not be applied")
		}
	}
}

func TestMigrator_Up_SQLExecutionError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr["ALTER TABLE"] = fmt.Errorf("SQL execution failed")
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	applied, err := m.Up()
	if err == nil {
		t.Error("Up() with SQL error should return error")
	}
	if len(applied) != 1 {
		t.Errorf("Up() applied %d before error, want 1", len(applied))
	}
}

func TestMigrator_Up_RecordError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr["INSERT INTO schema_migrations"] = fmt.Errorf("record failed")
	reg := NewRegistry()
	reg.MustRegister(&Migration{Version: 1, Description: "test", UpSQL: "CREATE TABLE t (id INT)"})
	m, _ := NewMigrator(reg, exec)

	_, err := m.Up()
	if err == nil {
		t.Error("Up() with record error should return error")
	}
}

func TestMigrator_Rollback_SQLExecutionError(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	exec.execErr["DROP TABLE posts"] = fmt.Errorf("rollback SQL failed")

	_, err := m.Rollback(1)
	if err == nil {
		t.Error("Rollback() with SQL error should return error")
	}
}

func TestMigrator_Rollback_RecordRemovalError(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	exec.execErr["DELETE FROM schema_migrations"] = fmt.Errorf("delete record failed")

	_, err := m.Rollback(1)
	if err == nil {
		t.Error("Rollback() with record removal error should return error")
	}
}

func TestMigrationLock_AcquireAndRelease(t *testing.T) {
	exec := newMockExecutor()
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	if err := lock.Acquire(); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if !lock.IsLocked() {
		t.Error("IsLocked() = false after Acquire()")
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if lock.IsLocked() {
		t.Error("IsLocked() = true after Release()")
	}
}

func TestMigrationLock_AcquireWhenLocked(t *testing.T) {
	exec := newMockExecutor()
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	_ = lock.Acquire()

	err := lock.Acquire()
	if !errors.Is(err, ErrLockAcquireFailed) {
		t.Errorf("Acquire() when already locked error = %v, want ErrLockAcquireFailed", err)
	}
}

func TestMigrationLock_ReleaseNotLocked(t *testing.T) {
	exec := newMockExecutor()
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	err := lock.Release()
	if err != nil {
		t.Errorf("Release() when not locked error = %v, want nil", err)
	}
}

func TestMigrationLock_ReleaseIdempotent(t *testing.T) {
	exec := newMockExecutor()
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	_ = lock.Acquire()
	_ = lock.Release()
	_ = lock.Release()
}

func TestMigrationLock_TryAcquire(t *testing.T) {
	exec := newMockExecutor()
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	err := lock.TryAcquire(5 * time.Second)
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !lock.IsLocked() {
		t.Error("IsLocked() = false after TryAcquire()")
	}
}

func TestMigrationLock_TryAcquireWhenLocked(t *testing.T) {
	exec := newMockExecutor()
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	_ = lock.Acquire()

	err := lock.TryAcquire(100 * time.Millisecond)
	if !errors.Is(err, ErrLockAcquireFailed) {
		t.Errorf("TryAcquire() when locked error = %v, want ErrLockAcquireFailed", err)
	}
}

func TestMigrationLock_AcquireWithCleanupError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr["DELETE FROM schema_migration_lock"] = fmt.Errorf("cleanup failed")
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	err := lock.Acquire()
	if !errors.Is(err, ErrLockAcquireFailed) {
		t.Errorf("Acquire() with cleanup error = %v, want ErrLockAcquireFailed", err)
	}
}

func TestMigrationLock_AcquireInsertError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr["INSERT INTO schema_migration_lock"] = fmt.Errorf("insert lock failed")
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	err := lock.Acquire()
	if !errors.Is(err, ErrLockAcquireFailed) {
		t.Errorf("Acquire() with insert error = %v, want ErrLockAcquireFailed", err)
	}
}

func TestMigrationLock_ReleaseDeleteError(t *testing.T) {
	exec := newMockExecutor()
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	_ = lock.Acquire()

	exec.execErr["DELETE FROM schema_migration_lock"] = fmt.Errorf("delete failed")

	err := lock.Release()
	if err == nil {
		t.Error("Release() with delete error should return error")
	}
}

func TestMigrator_FullLifecycle(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	applied, err := m.Up()
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if len(applied) != 3 {
		t.Fatalf("Up() applied %d, want 3", len(applied))
	}

	current, _ := m.CurrentVersion()
	if current != 3 {
		t.Errorf("CurrentVersion() = %d, want 3", current)
	}

	rolledBack, err := m.Rollback(1)
	if err != nil {
		t.Fatalf("Rollback(1) error = %v", err)
	}
	if len(rolledBack) != 2 {
		t.Fatalf("Rollback(1) rolled back %d, want 2", len(rolledBack))
	}

	current, _ = m.CurrentVersion()
	if current != 1 {
		t.Errorf("CurrentVersion() after rollback = %d, want 1", current)
	}

	applied2, err := m.Up()
	if err != nil {
		t.Fatalf("Up() after rollback error = %v", err)
	}
	if len(applied2) != 2 {
		t.Fatalf("Up() after rollback applied %d, want 2", len(applied2))
	}
	if applied2[0] != 2 || applied2[1] != 3 {
		t.Errorf("Up() after rollback versions = %v, want [2, 3]", applied2)
	}

	current, _ = m.CurrentVersion()
	if current != 3 {
		t.Errorf("CurrentVersion() after re-up = %d, want 3", current)
	}
}

func TestMigrator_RollbackThenUpTo(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	_, _ = m.Rollback(1)

	applied, err := m.UpTo(2)
	if err != nil {
		t.Fatalf("UpTo(2) after rollback error = %v", err)
	}
	if len(applied) != 1 || applied[0] != 2 {
		t.Errorf("UpTo(2) after rollback = %v, want [2]", applied)
	}

	current, _ := m.CurrentVersion()
	if current != 2 {
		t.Errorf("CurrentVersion() = %d, want 2", current)
	}
}

func TestMigrator_StatusDescriptions(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	statuses, _ := m.Status()
	expected := []struct {
		version     int
		description string
	}{
		{1, "create users table"},
		{2, "add email column"},
		{3, "create posts table"},
	}
	for i, exp := range expected {
		if statuses[i].Version != exp.version || statuses[i].Description != exp.description {
			t.Errorf("Status()[%d] = {Version:%d, Description:%q}, want {Version:%d, Description:%q}",
				i, statuses[i].Version, statuses[i].Description, exp.version, exp.description)
		}
	}
}

func TestMigrator_QueryError(t *testing.T) {
	exec := newMockExecutor()
	exec.queryErr["SELECT version, applied_at"] = fmt.Errorf("query failed")
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, err := m.Status()
	if err == nil {
		t.Error("Status() with query error should return error")
	}
}

func TestMigrator_CurrentVersion_QueryError(t *testing.T) {
	exec := newMockExecutor()
	exec.queryErr["SELECT version, applied_at"] = fmt.Errorf("query failed")
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, err := m.CurrentVersion()
	if err == nil {
		t.Error("CurrentVersion() with query error should return error")
	}
}

func TestMigrator_Up_CreateTableError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr["CREATE TABLE IF NOT EXISTS schema_migrations"] = fmt.Errorf("create table failed")
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, err := m.Up()
	if err == nil {
		t.Error("Up() with create table error should return error")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup
	numGoroutines := 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			v := id + 1
			_ = r.Register(&Migration{
				Version:     v,
				Description: fmt.Sprintf("migration_%d", v),
				UpSQL:       fmt.Sprintf("SQL_%d", v),
			})
		}(i)
	}
	wg.Wait()

	versions := r.Versions()
	if len(versions) != numGoroutines {
		t.Errorf("Versions() length = %d, want %d", len(versions), numGoroutines)
	}
}

func TestMigrator_ConcurrentUp(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	var wg sync.WaitGroup
	numGoroutines := 5

	results := make([][]int, numGoroutines)
	errs := make([]error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			applied, err := m.Up()
			results[idx] = applied
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	successCount := 0
	for i, err := range errs {
		if err == nil {
			successCount++
		} else {
			if !errors.Is(err, ErrLockAcquireFailed) {
				t.Errorf("goroutine %d error = %v, want ErrLockAcquireFailed or nil", i, err)
			}
		}
	}

	if successCount == 0 {
		t.Error("at least one goroutine should succeed")
	}
}

func TestMigrationLock_ConcurrentAcquire(t *testing.T) {
	exec := newMockExecutor()
	lock := &MigrationLock{
		executor:  exec,
		lockTable: DefaultLockTable,
		timeout:   DefaultLockTimeout,
	}

	var wg sync.WaitGroup
	numGoroutines := 5
	acquired := make([]bool, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			err := lock.Acquire()
			if err == nil {
				acquired[idx] = true
				time.Sleep(10 * time.Millisecond)
				_ = lock.Release()
			}
		}(i)
	}
	wg.Wait()

	acquiredCount := 0
	for _, a := range acquired {
		if a {
			acquiredCount++
		}
	}
	if acquiredCount == 0 {
		t.Error("at least one goroutine should acquire lock")
	}
}

func TestMigrator_Up_SQLErrorOnSecondMigration(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr["ALTER TABLE users ADD COLUMN"] = fmt.Errorf("column exists")
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	applied, err := m.Up()
	if err == nil {
		t.Error("Up() with SQL error should return error")
	}
	if len(applied) != 1 {
		t.Errorf("Up() applied %d before error, want 1", len(applied))
	}

	current, _ := m.CurrentVersion()
	if current != 1 {
		t.Errorf("CurrentVersion() = %d, want 1 after partial failure", current)
	}
}

func TestMigrator_Rollback_ReverseOrder(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	rolledBack, err := m.Rollback(0)
	if err != nil {
		t.Fatalf("Rollback(0) error = %v", err)
	}
	if len(rolledBack) != 3 {
		t.Fatalf("Rollback(0) length = %d, want 3", len(rolledBack))
	}
	if rolledBack[0] != 3 {
		t.Errorf("Rollback(0)[0] = %d, want 3 (reverse order)", rolledBack[0])
	}
	if rolledBack[1] != 2 {
		t.Errorf("Rollback(0)[1] = %d, want 2 (reverse order)", rolledBack[1])
	}
	if rolledBack[2] != 1 {
		t.Errorf("Rollback(0)[2] = %d, want 1 (reverse order)", rolledBack[2])
	}
}

func TestMigrator_Rollback_PartialRollback(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	rolledBack, err := m.Rollback(2)
	if err != nil {
		t.Fatalf("Rollback(2) error = %v", err)
	}
	if len(rolledBack) != 1 {
		t.Fatalf("Rollback(2) length = %d, want 1", len(rolledBack))
	}
	if rolledBack[0] != 3 {
		t.Errorf("Rollback(2)[0] = %d, want 3", rolledBack[0])
	}

	current, _ := m.CurrentVersion()
	if current != 2 {
		t.Errorf("CurrentVersion() after partial rollback = %d, want 2", current)
	}
}

func TestMigrator_MultipleRollbackSteps(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	_, _ = m.Rollback(2)

	_, err := m.Rollback(0)
	if err != nil {
		t.Fatalf("Rollback(0) error = %v", err)
	}

	current, _ := m.CurrentVersion()
	if current != 0 {
		t.Errorf("CurrentVersion() = %d, want 0", current)
	}
}

func TestMigrator_LockReleasedAfterUp(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	if m.lock.IsLocked() {
		t.Error("lock should be released after Up()")
	}
}

func TestMigrator_LockReleasedAfterRollback(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()
	_, _ = m.Rollback(1)

	if m.lock.IsLocked() {
		t.Error("lock should be released after Rollback()")
	}
}

func TestMigrator_LockReleasedOnUpError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr["ALTER TABLE"] = fmt.Errorf("SQL error")
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()

	if m.lock.IsLocked() {
		t.Error("lock should be released even when Up() fails")
	}
}

func TestMigrator_EnsureLockTableError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr["CREATE TABLE IF NOT EXISTS schema_migration_lock"] = fmt.Errorf("create lock table failed")
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, err := m.Up()
	if err == nil {
		t.Error("Up() with lock table creation error should return error")
	}
}

func TestMigrator_UpWithLockError(t *testing.T) {
	exec := newMockExecutor()
	exec.execErr["INSERT INTO schema_migration_lock"] = fmt.Errorf("lock failed")
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, err := m.Up()
	if err == nil {
		t.Error("Up() with lock error should return error")
	}
}

func TestMigration_VersionOrder(t *testing.T) {
	exec := newMockExecutor()
	reg := NewRegistry()
	reg.MustRegister(&Migration{Version: 5, Description: "v5", UpSQL: "SQL5", DownSQL: "DOWN5"})
	reg.MustRegister(&Migration{Version: 2, Description: "v2", UpSQL: "SQL2", DownSQL: "DOWN2"})
	reg.MustRegister(&Migration{Version: 8, Description: "v8", UpSQL: "SQL8", DownSQL: "DOWN8"})

	m, _ := NewMigrator(reg, exec)

	applied, err := m.Up()
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if len(applied) != 3 {
		t.Fatalf("Up() applied %d, want 3", len(applied))
	}
	if applied[0] != 2 || applied[1] != 5 || applied[2] != 8 {
		t.Errorf("Up() order = %v, want [2, 5, 8]", applied)
	}
}

func TestMigrator_Up_PartialThenMore(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	applied1, _ := m.UpTo(1)
	if len(applied1) != 1 || applied1[0] != 1 {
		t.Errorf("UpTo(1) = %v, want [1]", applied1)
	}

	applied2, _ := m.UpTo(2)
	if len(applied2) != 1 || applied2[0] != 2 {
		t.Errorf("UpTo(2) after UpTo(1) = %v, want [2]", applied2)
	}

	applied3, _ := m.Up()
	if len(applied3) != 1 || applied3[0] != 3 {
		t.Errorf("Up() after UpTo(2) = %v, want [3]", applied3)
	}
}

func TestMigration_SingleMigration(t *testing.T) {
	exec := newMockExecutor()
	reg := NewRegistry()
	reg.MustRegister(&Migration{Version: 1, Description: "only one", UpSQL: "CREATE TABLE t (id INT)", DownSQL: "DROP TABLE t"})
	m, _ := NewMigrator(reg, exec)

	applied, err := m.Up()
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	if len(applied) != 1 || applied[0] != 1 {
		t.Errorf("Up() = %v, want [1]", applied)
	}

	rolledBack, err := m.Rollback(0)
	if err != nil {
		t.Fatalf("Rollback(0) error = %v", err)
	}
	if len(rolledBack) != 1 || rolledBack[0] != 1 {
		t.Errorf("Rollback(0) = %v, want [1]", rolledBack)
	}
}

func TestMigrator_Status_VersionOrder(t *testing.T) {
	exec := newMockExecutor()
	reg := NewRegistry()
	reg.MustRegister(&Migration{Version: 3, Description: "v3", UpSQL: "SQL3"})
	reg.MustRegister(&Migration{Version: 1, Description: "v1", UpSQL: "SQL1"})
	reg.MustRegister(&Migration{Version: 2, Description: "v2", UpSQL: "SQL2"})

	m, _ := NewMigrator(reg, exec)
	statuses, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(statuses) != 3 {
		t.Fatalf("Status() length = %d, want 3", len(statuses))
	}
	for i, s := range statuses {
		if s.Version != i+1 {
			t.Errorf("Status()[%d].Version = %d, want %d", i, s.Version, i+1)
		}
	}
}

func TestRegistry_VersionsReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Migration{Version: 1, Description: "v1", UpSQL: "SQL1"})

	v1 := r.Versions()
	v1[0] = 999

	v2 := r.Versions()
	if v2[0] == 999 {
		t.Error("Versions() should return a copy, not a reference")
	}
}

func TestRegistry_AllReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Migration{Version: 1, Description: "v1", UpSQL: "SQL1"})

	all1 := r.All()
	all1[0].Version = 999

	all2 := r.All()
	if all2[0].Version == 999 {
		t.Error("All() should return independent copies")
	}
}

func TestMigrator_Rollback_UpAfterFullRollback(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()
	_, _ = m.Rollback(0)

	current, _ := m.CurrentVersion()
	if current != 0 {
		t.Errorf("CurrentVersion() after full rollback = %d, want 0", current)
	}

	applied, err := m.Up()
	if err != nil {
		t.Fatalf("Up() after full rollback error = %v", err)
	}
	if len(applied) != 3 {
		t.Fatalf("Up() after full rollback applied %d, want 3", len(applied))
	}
}

func TestMigrator_Rollback_PreserveLowerVersions(t *testing.T) {
	exec := newMockExecutor()
	reg := newTestRegistry()
	m, _ := NewMigrator(reg, exec)

	_, _ = m.Up()
	_, _ = m.Rollback(1)

	statuses, _ := m.Status()

	appliedCount := 0
	for _, s := range statuses {
		if s.Applied {
			appliedCount++
		}
	}
	if appliedCount != 1 {
		t.Errorf("applied count after rollback = %d, want 1", appliedCount)
	}

	if !statuses[0].Applied {
		t.Error("version 1 should still be applied after rollback to 1")
	}
}

func TestMigrationLock_DefaultTimeout(t *testing.T) {
	if DefaultLockTimeout != 30*time.Second {
		t.Errorf("DefaultLockTimeout = %v, want 30s", DefaultLockTimeout)
	}
}

func TestMigrationLock_DefaultTables(t *testing.T) {
	if DefaultMigrationsTable != "schema_migrations" {
		t.Errorf("DefaultMigrationsTable = %q, want %q", DefaultMigrationsTable, "schema_migrations")
	}
	if DefaultLockTable != "schema_migration_lock" {
		t.Errorf("DefaultLockTable = %q, want %q", DefaultLockTable, "schema_migration_lock")
	}
}

func TestMigrator_Rollback_PartiallyAppliedMigrations(t *testing.T) {
	exec := newMockExecutor()
	reg := NewRegistry()
	reg.MustRegister(&Migration{Version: 1, Description: "v1", UpSQL: "CREATE TABLE t1", DownSQL: "DROP TABLE t1"})
	reg.MustRegister(&Migration{Version: 2, Description: "v2", UpSQL: "CREATE TABLE t2", DownSQL: "DROP TABLE t2"})
	reg.MustRegister(&Migration{Version: 3, Description: "v3", UpSQL: "CREATE TABLE t3", DownSQL: "DROP TABLE t3"})
	reg.MustRegister(&Migration{Version: 4, Description: "v4", UpSQL: "CREATE TABLE t4", DownSQL: "DROP TABLE t4"})
	reg.MustRegister(&Migration{Version: 5, Description: "v5", UpSQL: "CREATE TABLE t5", DownSQL: "DROP TABLE t5"})

	m, _ := NewMigrator(reg, exec)

	_, _ = m.UpTo(3)

	current, _ := m.CurrentVersion()
	if current != 3 {
		t.Fatalf("CurrentVersion() after UpTo(3) = %d, want 3", current)
	}

	exec.mu.Lock()
	exec.execLog = nil
	exec.mu.Unlock()

	rolledBack, err := m.Rollback(1)
	if err != nil {
		t.Fatalf("Rollback(1) error = %v", err)
	}

	if len(rolledBack) != 2 {
		t.Fatalf("Rollback(1) rolled back %d, want 2", len(rolledBack))
	}
	if rolledBack[0] != 3 || rolledBack[1] != 2 {
		t.Errorf("Rollback(1) versions = %v, want [3, 2]", rolledBack)
	}

	exec.mu.Lock()

	for _, record := range exec.execLog {
		if containsPattern(record.query, "DROP TABLE") {
			if containsPattern(record.query, "t4") || containsPattern(record.query, "t5") {
				t.Errorf("should not rollback v4 or v5, but executed: %s", record.query)
			}
		}
	}
	exec.mu.Unlock()

	current, _ = m.CurrentVersion()
	if current != 1 {
		t.Errorf("CurrentVersion() after rollback = %d, want 1", current)
	}
}

func TestMigrator_Rollback_PartiallyApplied_SkipUnapplied(t *testing.T) {
	exec := newMockExecutor()
	reg := NewRegistry()
	reg.MustRegister(&Migration{Version: 1, Description: "v1", UpSQL: "SQL1", DownSQL: "DOWN1"})
	reg.MustRegister(&Migration{Version: 2, Description: "v2", UpSQL: "SQL2", DownSQL: "DOWN2"})
	reg.MustRegister(&Migration{Version: 3, Description: "v3", UpSQL: "SQL3", DownSQL: "DOWN3"})
	reg.MustRegister(&Migration{Version: 4, Description: "v4", UpSQL: "SQL4", DownSQL: "DOWN4"})
	reg.MustRegister(&Migration{Version: 5, Description: "v5", UpSQL: "SQL5", DownSQL: "DOWN5"})

	m, _ := NewMigrator(reg, exec)

	_, _ = m.UpTo(2)

	exec.mu.Lock()
	exec.execLog = nil
	exec.mu.Unlock()

	rolledBack, err := m.Rollback(0)
	if err != nil {
		t.Fatalf("Rollback(0) error = %v", err)
	}

	if len(rolledBack) != 2 {
		t.Fatalf("Rollback(0) rolled back %d, want 2", len(rolledBack))
	}
	if rolledBack[0] != 2 || rolledBack[1] != 1 {
		t.Errorf("Rollback(0) versions = %v, want [2, 1]", rolledBack)
	}

	exec.mu.Lock()
	defer exec.mu.Unlock()

	downCount := 0
	for _, record := range exec.execLog {
		if containsPattern(record.query, "DOWN") {
			downCount++
			if containsPattern(record.query, "DOWN3") || containsPattern(record.query, "DOWN4") || containsPattern(record.query, "DOWN5") {
				t.Errorf("should not execute DOWN3/4/5, but executed: %s", record.query)
			}
		}
	}
	if downCount != 2 {
		t.Errorf("executed %d DOWN SQL statements, want 2", downCount)
	}
}

func TestRegistry_RangeReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&Migration{Version: 1, Description: "v1", UpSQL: "SQL1"})
	r.MustRegister(&Migration{Version: 2, Description: "v2", UpSQL: "SQL2"})
	r.MustRegister(&Migration{Version: 3, Description: "v3", UpSQL: "SQL3"})

	rng := r.Range(1, 2)
	if len(rng) != 2 {
		t.Fatalf("Range(1,2) length = %d, want 2", len(rng))
	}

	rng[0].Description = "MODIFIED"
	rng[1].UpSQL = "MODIFIED_SQL"

	all := r.All()
	if all[0].Description == "MODIFIED" {
		t.Error("Range() should return a copy, but modification affected registry")
	}
	if all[1].UpSQL == "MODIFIED_SQL" {
		t.Error("Range() should return a copy, but modification affected registry")
	}
}

func TestMigrator_Rollback_UnappliedVersionsIgnored(t *testing.T) {
	exec := newMockExecutor()
	reg := NewRegistry()
	reg.MustRegister(&Migration{Version: 1, Description: "v1", UpSQL: "UP1", DownSQL: "DOWN1"})
	reg.MustRegister(&Migration{Version: 2, Description: "v2", UpSQL: "UP2", DownSQL: "DOWN2"})
	reg.MustRegister(&Migration{Version: 3, Description: "v3", UpSQL: "UP3", DownSQL: "DOWN3"})
	reg.MustRegister(&Migration{Version: 4, Description: "v4", UpSQL: "UP4", DownSQL: "DOWN4"})

	m, _ := NewMigrator(reg, exec)

	_, _ = m.UpTo(1)

	current, _ := m.CurrentVersion()
	if current != 1 {
		t.Fatalf("CurrentVersion() = %d, want 1", current)
	}

	rolledBack, err := m.Rollback(1)
	if err != nil {
		t.Fatalf("Rollback(1) error = %v", err)
	}
	if len(rolledBack) != 0 {
		t.Errorf("Rollback(1) when current=1 should roll back 0, got %d", len(rolledBack))
	}

	_, err = m.Rollback(0)
	if err != nil {
		t.Fatalf("Rollback(0) error = %v", err)
	}

	exec.mu.Lock()
	defer exec.mu.Unlock()

	for _, record := range exec.execLog {
		if containsPattern(record.query, "DOWN2") || containsPattern(record.query, "DOWN3") || containsPattern(record.query, "DOWN4") {
			t.Errorf("should not execute DOWN for unapplied migrations, but got: %s", record.query)
		}
	}
}

func TestMigrator_Rollback_NoDownSQLOnlyForApplied(t *testing.T) {
	exec := newMockExecutor()
	reg := NewRegistry()
	reg.MustRegister(&Migration{Version: 1, Description: "v1", UpSQL: "UP1", DownSQL: "DOWN1"})
	reg.MustRegister(&Migration{Version: 2, Description: "v2", UpSQL: "UP2", DownSQL: ""})
	reg.MustRegister(&Migration{Version: 3, Description: "v3", UpSQL: "UP3", DownSQL: "DOWN3"})

	m, _ := NewMigrator(reg, exec)

	_, _ = m.UpTo(1)

	_, err := m.Rollback(0)
	if err != nil {
		t.Fatalf("Rollback(0) error = %v", err)
	}

	current, _ := m.CurrentVersion()
	if current != 0 {
		t.Errorf("CurrentVersion() = %d, want 0", current)
	}
}
