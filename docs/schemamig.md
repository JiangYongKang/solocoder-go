# Schema 迁移工具模块

## 模块功能概述

Schema 迁移工具（schemamig）是一个 Go 语言数据库 Schema 版本化管理与迁移执行模块，位于 `internal/schemamig/` 包下。它负责管理数据库迁移脚本的生命周期，包括注册、正向执行、回滚和并发防护，为上层应用提供可靠、幂等、可回溯的 Schema 变更能力。核心功能包括：

1. **迁移脚本版本化管理**：迁移脚本按版本号顺序管理，每个脚本包含正向执行 SQL（UpSQL）和回滚 SQL（DownSQL）。支持查询所有已定义的迁移脚本及其执行状态（已应用/未应用/应用时间）。

2. **正向执行**：支持按版本号顺序依次执行尚未应用的迁移脚本，记录每个脚本的执行状态和执行时间。已成功执行的脚本不会重复执行，保证幂等性。

3. **回滚操作**：支持将 Schema 回滚到指定的历史版本，按版本号逆序依次执行各版本的回滚 SQL。回滚前校验目标版本不高于当前已执行的版本，防止无效回滚。

4. **迁移锁防并发执行**：在执行迁移前获取全局迁移锁，执行完成后释放锁，确保同一时刻只有一个进程在执行迁移操作。锁支持超时自动释放防止死锁。

---

## 核心结构体与职责

### Migration

```go
type Migration struct {
    Version     int    // 迁移版本号，必须为正整数，按升序排列
    Description string // 迁移描述信息
    UpSQL       string // 正向执行 SQL（不可为空）
    DownSQL     string // 回滚 SQL（可为空，但回滚时会报错）
}
```

**职责**：表示单个迁移脚本的定义。`Version` 是迁移的唯一标识，必须全局唯一且建议从 1 开始连续递增。`UpSQL` 是执行迁移时运行的 SQL，`DownSQL` 是回滚迁移时运行的 SQL。

---

### MigrationStatus

```go
type MigrationStatus struct {
    Version     int       // 迁移版本号
    Description string    // 迁移描述信息
    Applied     bool      // 是否已应用
    AppliedAt   time.Time // 应用时间（未应用时为零值）
}
```

**职责**：表示迁移脚本的执行状态快照，由 `Migrator.Status()` 方法返回，供上层查询当前所有迁移的执行情况。

---

### SQLExecutor

```go
type SQLExecutor interface {
    Exec(query string, args ...interface{}) error
    Query(query string, args ...interface{}) (Rows, error)
}
```

**职责**：数据库操作抽象接口。上层需要提供实现了该接口的数据库连接对象，模块通过该接口执行 SQL 语句和查询。这种抽象设计使模块与具体数据库驱动解耦，便于单元测试中使用 mock 实现。

---

### Rows

```go
type Rows interface {
    Next() bool
    Scan(dest ...interface{}) error
    Close() error
}
```

**职责**：查询结果集抽象接口，与标准 `database/sql.Rows` 行为一致。

---

### Registry

```go
type Registry struct {
    // 内部字段省略
}
```

**职责**：迁移脚本注册中心，管理所有已定义的迁移脚本。

对外方法：
- `NewRegistry() *Registry`：创建新的注册中心
- `Register(m *Migration) error`：注册一个迁移脚本，校验版本号唯一性、正整数约束、UpSQL 非空
- `MustRegister(m *Migration)`：注册迁移脚本，失败则 panic
- `Get(version int) (*Migration, bool)`：按版本号获取迁移脚本
- `Versions() []int`：返回所有已注册的版本号（有序副本）
- `All() []*Migration`：返回所有已注册的迁移脚本（按版本号升序，返回副本）
- `Validate() error`：校验版本号是否从 1 开始且连续无间隔
- `Range(from, to int) []*Migration`：返回指定版本范围内的迁移脚本

---

### Migrator

```go
type Migrator struct {
    // 内部字段省略
}
```

**职责**：迁移执行引擎，负责正向执行、回滚、状态查询和锁管理。

构造函数：
- `NewMigrator(registry *Registry, executor SQLExecutor, opts ...MigratorOption) (*Migrator, error)`：创建迁移器

配置选项：
- `WithTableName(name string) MigratorOption`：自定义迁移记录表名（默认 `schema_migrations`）
- `WithLockTable(name string) MigratorOption`：自定义锁表名（默认 `schema_migration_lock`）
- `WithLockTimeout(d time.Duration) MigratorOption`：自定义锁超时时间（默认 30 秒）

对外方法：
- `Up() ([]int, error)`：执行所有未应用的迁移，返回已应用的版本号列表
- `UpTo(targetVersion int) ([]int, error)`：执行到指定版本的未应用迁移
- `Rollback(targetVersion int) ([]int, error)`：回滚到指定版本，返回已回滚的版本号列表
- `CurrentVersion() (int, error)`：获取当前已应用的最高版本号（无应用记录时返回 0）
- `Status() ([]*MigrationStatus, error)`：查询所有迁移脚本的执行状态

---

### MigrationLock

```go
type MigrationLock struct {
    // 内部字段省略
}
```

**职责**：迁移锁管理器，防止多个进程同时执行迁移操作。

方法：
- `Acquire() error`：获取迁移锁，锁已被持有时返回 `ErrLockAcquireFailed`
- `Release() error`：释放迁移锁（未持有时调用为空操作）
- `IsLocked() bool`：查询锁是否被持有
- `TryAcquire(timeout time.Duration) error`：在超时时间内尝试获取锁，超时返回 `ErrLockTimeout`

---

### 错误类型

| 错误变量 | 含义 |
|---------|------|
| `ErrDuplicateVersion` | 注册了重复版本号的迁移脚本 |
| `ErrVersionGap` | 版本号不连续（未从 1 开始或中间有间隔） |
| `ErrMigrationNotFound` | 指定版本的迁移脚本不存在 |
| `ErrLockAcquireFailed` | 获取迁移锁失败（已被其他进程持有） |
| `ErrLockTimeout` | 尝试获取迁移锁超时 |
| `ErrAlreadyApplied` | 迁移已应用（内部使用） |
| `ErrRollbackTarget` | 回滚目标版本高于当前版本 |
| `ErrNoMigrations` | 注册中心无迁移脚本 |
| `ErrNotApplied` | 迁移未应用，无法回滚 |
| `ErrEmptyUpSQL` | 迁移的 UpSQL 为空 |
| `ErrInvalidVersion` | 迁移版本号无效（非正整数） |

---

## 迁移脚本从创建到执行的完整生命周期

### 一、注册阶段

```
NewRegistry()
      │
      ▼
  Registry.Register(&Migration{Version:1, UpSQL:"...", DownSQL:"..."})
      │
      ├─ 校验 Version > 0 ──否──► 返回 ErrInvalidVersion
      ├─ 校验 UpSQL != "" ──否──► 返回 ErrEmptyUpSQL
      ├─ 校验版本号唯一 ──否──► 返回 ErrDuplicateVersion
      │
      ▼
  写入 migrations map 和 versions 切片
  对 versions 排序
      │
      ▼
  继续注册更多迁移脚本 ...
      │
      ▼
  Registry.Validate()
      │
      ├─ 检查首版本为 1 ──否──► 返回 ErrVersionGap
      └─ 检查版本号连续 ──否──► 返回 ErrVersionGap
```

### 二、正向执行阶段（Up）

```
Migrator.Up() 调用
      │
      ▼
  加互斥锁 mu.Lock()
      │
      ▼
  ensureMigrationsTable() ── 创建 schema_migrations 表
      │
      ▼
  ensureLockTable() ── 创建 schema_migration_lock 表
      │
      ▼
  lock.Acquire() ── 获取迁移锁
      ├─ 清理过期的锁记录（locked_at < now - timeout）
      ├─ 尝试插入锁记录
      └─ 失败 ──► 返回 ErrLockAcquireFailed
      │
      ▼
  getAppliedVersions() ── 查询已应用的版本
      │
      ▼
  遍历所有迁移脚本（按版本号升序）：
      │
      ├─ 版本已应用？──是──► 跳过
      │
      ├─ executor.Exec(mig.UpSQL) ── 执行正向 SQL
      │   └─ 失败 ──► 释放锁，返回错误
      │
      └─ recordApplied(version) ── 记录执行状态
          └─ 失败 ──► 释放锁，返回错误
      │
      ▼
  lock.Release() ── 释放迁移锁
      │
      ▼
  返回已应用的版本号列表
```

### 三、回滚阶段（Rollback）

```
Migrator.Rollback(targetVersion) 调用
      │
      ▼
  加互斥锁 mu.Lock()
      │
      ▼
  ensureMigrationsTable() + ensureLockTable()
      │
      ▼
  lock.Acquire() ── 获取迁移锁
      │
      ▼
  CurrentVersion() ── 获取当前最高版本号
      │
      ▼
  targetVersion > currentVersion？──是──► 返回 ErrRollbackTarget
      │否
      ▼
  targetVersion == currentVersion？──是──► 返回空列表（无需回滚）
      │否
      ▼
  逆序遍历迁移脚本（从最高版本到 targetVersion+1）：
      │
      ├─ DownSQL 为空？──是──► 返回错误
      │
      ├─ executor.Exec(mig.DownSQL) ── 执行回滚 SQL
      │   └─ 失败 ──► 释放锁，返回错误
      │
      └─ removeApplied(version) ── 删除执行记录
          └─ 失败 ──► 释放锁，返回错误
      │
      ▼
  lock.Release() ── 释放迁移锁
      │
      ▼
  返回已回滚的版本号列表（按回滚顺序排列，即逆序）
```

### 四、状态查询阶段

```
Migrator.Status() 调用
      │
      ▼
  ensureMigrationsTable()
      │
      ▼
  getAppliedVersions() ── 查询已应用的版本及其时间
      │
      ▼
  遍历所有注册的迁移脚本：
      ├─ 已应用？──► Applied=true, AppliedAt=记录的时间
      └─ 未应用？──► Applied=false, AppliedAt=零值
      │
      ▼
  返回 MigrationStatus 列表
```

---

## 数据库表结构

### schema_migrations（迁移记录表）

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER NOT NULL PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL
);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| version | INTEGER | 迁移版本号，主键 |
| applied_at | TIMESTAMP | 迁移应用时间 |

### schema_migration_lock（迁移锁表）

```sql
CREATE TABLE IF NOT EXISTS schema_migration_lock (
    id        INTEGER NOT NULL PRIMARY KEY,
    locked_at TIMESTAMP NOT NULL,
    locked_by VARCHAR(255) NOT NULL
);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 锁 ID（固定为 1，全局单锁） |
| locked_at | TIMESTAMP | 加锁时间，用于超时判断 |
| locked_by | VARCHAR(255) | 持锁者标识（进程 ID + 时间戳） |

---

## 迁移锁机制详解

### 锁获取流程

1. **清理过期锁**：执行 `DELETE FROM schema_migration_lock WHERE locked_at < (now - timeout)`，自动释放超时的锁记录，防止因进程崩溃导致的死锁。
2. **插入锁记录**：执行 `INSERT INTO schema_migration_lock (id, locked_at, locked_by) VALUES (1, now, identifier)`。如果表中已存在 `id=1` 的记录（即锁已被持有），则插入失败，返回 `ErrLockAcquireFailed`。
3. **记录锁状态**：在内存中标记锁为已持有状态。

### 锁释放流程

1. 执行 `DELETE FROM schema_migration_lock WHERE id = 1`，删除锁记录。
2. 清除内存中的锁状态标记。

### 超时自动释放

当获取锁时，会先清理 `locked_at` 早于 `now - timeout` 的锁记录。这意味着如果持锁进程崩溃未释放锁，锁记录会在超时后被后续的锁获取操作自动清理，从而防止死锁。默认超时时间为 30 秒。

### TryAcquire 带超时重试

`TryAcquire(timeout)` 方法在指定超时时间内反复尝试获取锁，每次尝试间隔 50ms。如果锁被其他进程持有但在超时内释放，则可以成功获取；否则返回 `ErrLockTimeout`。

---

## 使用示例

### 示例 1：基础使用 — 注册并执行迁移

```go
package main

import (
    "database/sql"
    "log"
    "solocoder-go/internal/schemamig"
)

func main() {
    db, _ := sql.Open("mysql", "user:pass@/mydb")
    defer db.Close()

    registry := schemamig.NewRegistry()
    registry.MustRegister(&schemamig.Migration{
        Version:     1,
        Description: "create users table",
        UpSQL:       "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100))",
        DownSQL:     "DROP TABLE users",
    })
    registry.MustRegister(&schemamig.Migration{
        Version:     2,
        Description: "add email column",
        UpSQL:       "ALTER TABLE users ADD COLUMN email VARCHAR(255)",
        DownSQL:     "ALTER TABLE users DROP COLUMN email",
    })

    migrator, err := schemamig.NewMigrator(registry, db)
    if err != nil {
        log.Fatal(err)
    }

    applied, err := migrator.Up()
    if err != nil {
        log.Fatalf("migration failed: %v", err)
    }
    log.Printf("applied versions: %v", applied)
}
```

### 示例 2：部分执行与回滚

```go
migrator, _ := schemamig.NewMigrator(registry, db)

applied, _ := migrator.UpTo(1)
log.Printf("applied up to v1: %v", applied)

current, _ := migrator.CurrentVersion()
log.Printf("current version: %d", current) // 输出: 1

rolledBack, err := migrator.Rollback(0)
if err != nil {
    log.Fatalf("rollback failed: %v", err)
}
log.Printf("rolled back versions: %v", rolledBack)

current, _ = migrator.CurrentVersion()
log.Printf("current version after rollback: %d", current) // 输出: 0
```

### 示例 3：查询迁移状态

```go
migrator, _ := schemamig.NewMigrator(registry, db)

statuses, err := migrator.Status()
if err != nil {
    log.Fatal(err)
}
for _, s := range statuses {
    if s.Applied {
        log.Printf("v%d [%s]: applied at %s", s.Version, s.Description, s.AppliedAt)
    } else {
        log.Printf("v%d [%s]: pending", s.Version, s.Description)
    }
}
```

### 示例 4：自定义配置

```go
migrator, err := schemamig.NewMigrator(registry, db,
    schemamig.WithTableName("my_migrations"),
    schemamig.WithLockTable("my_migration_lock"),
    schemamig.WithLockTimeout(60*time.Second),
)
if err != nil {
    log.Fatal(err)
}
```

### 示例 5：版本号校验

```go
registry := schemamig.NewRegistry()
registry.MustRegister(&schemamig.Migration{Version: 1, Description: "v1", UpSQL: "SQL1"})
registry.MustRegister(&schemamig.Migration{Version: 3, Description: "v3", UpSQL: "SQL3"})

if err := registry.Validate(); err != nil {
    log.Printf("version validation failed: %v", err)
    // 输出: version gap detected between version 1 and 3
}
```

---

## 单元测试覆盖

模块包含 56 个单元测试，覆盖以下场景：

| 类别 | 测试内容 |
|------|---------|
| Registry 注册 | 正常注册、重复版本号、无效版本号、空 UpSQL、MustRegister panic |
| Registry 查询 | Get 存在/不存在、All 排序、Versions 副本、All 副本、Range 范围查询 |
| Registry 校验 | 空注册表、连续版本、不从 1 开始、中间有间隔 |
| Registry 并发 | 多 goroutine 并发注册 |
| Migrator 创建 | nil registry、nil executor、自定义选项 |
| 正向执行 | 全部执行、无待执行、幂等性、无迁移脚本、部分执行 UpTo |
| 回滚操作 | 正常回滚、回滚到 0、相同版本、目标高于当前、无已应用迁移、无 DownSQL |
| 生命周期 | 完整 Up→Rollback→Up、RollbackThenUpTo、UpAfterFullRollback |
| 状态查询 | 全部/部分应用、描述信息、版本号排序 |
| SQL 错误 | Up SQL 执行错误、记录写入错误、Rollback SQL 错误、记录删除错误 |
| 迁移锁 | Acquire/Release、重复获取、未锁定时释放、幂等释放、TryAcquire |
| 迁移锁错误 | 清理过期锁错误、插入锁记录错误、删除锁记录错误 |
| 并发安全 | 多 goroutine 并发 Up、多 goroutine 并发锁获取 |
| 锁释放保证 | Up 后释放、Rollback 后释放、Up 出错后释放 |
| 边界条件 | 单条迁移、版本号乱序注册、部分执行后继续执行 |
| 默认值 | 默认表名、默认锁超时时间 |
