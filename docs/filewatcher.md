# 文件变更监控器 (FileWatcher) 模块需求文档

## 1. 模块概述

文件变更监控器是一个基于轮询机制的跨平台文件系统监听组件，用于监控指定目录及其所有子目录下的文件变更事件。通过定期扫描目录树、比对文件状态快照来检测文件的创建、修改和删除操作，并支持防抖去重、事件过滤等高级特性。

本模块适用于配置文件热加载、源代码变更监听、自动化构建触发等需要实时感知文件系统变化的场景。模块采用纯标准库实现，无需外部依赖，具有良好的跨平台兼容性。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 递归目录监听 | 支持注册一个根目录，监听器自动递归监听该目录及其所有子目录下的文件变更 |
| F2 | 创建事件回调 | 当监听目录内有新文件被创建时，触发注册的创建事件回调函数 |
| F3 | 修改事件回调 | 当监听目录内有文件被修改时，触发注册的修改事件回调函数 |
| F4 | 删除事件回调 | 当监听目录内有文件被删除时，触发注册的删除事件回调函数 |
| F5 | 防抖去重 | 对短时间内同一文件的同类事件进行合并去重，时间窗口内只触发一次回调 |
| F6 | 文件扩展名过滤 | 支持配置文件扩展名白名单，只有匹配的文件才会触发事件 |
| F7 | 文件名模式过滤 | 支持配置文件名 glob 模式，只有匹配的文件才会触发事件 |
| F8 | 目录排除过滤 | 支持配置需要排除的目录名，这些目录及其子目录将被跳过 |
| F9 | 包含路径过滤 | 支持配置完整路径的 glob 模式，只有匹配路径的文件才会触发事件 |
| F10 | 生命周期管理 | 提供 Start/Stop 方法控制监听器的启动和停止，支持幂等调用 |
| F11 | 状态查询 | 提供运行状态、监听目录、已监听文件数等查询接口 |
| F12 | 初始状态快照 | 监听启动时扫描已有文件作为基线，不触发初始文件的创建事件 |

## 3. 核心结构体与职责

### 3.1 EventType - 事件类型枚举

```go
type EventType int

const (
    EventCreate EventType = iota
    EventModify
    EventDelete
)
```

**主要职责：**
- 定义三种文件事件类型：创建、修改、删除
- 提供 `String()` 方法用于类型的可读化输出

### 3.2 Event - 文件事件

```go
type Event struct {
    Type EventType
    Path string
}
```

**主要职责：**
- 封装单个文件变更事件的完整信息
- `Type` 字段标识事件类型（创建/修改/删除）
- `Path` 字段存储发生变更的文件的绝对路径

### 3.3 EventCallback - 事件回调函数类型

```go
type EventCallback func(event Event)
```

**主要职责：**
- 定义事件回调函数的签名
- 回调函数接收一个 `Event` 参数，包含事件类型和文件路径
- 调用方通过注册不同类型的回调函数来响应不同的文件事件

### 3.4 FilterConfig - 过滤规则配置

```go
type FilterConfig struct {
    FilePatterns    []string
    FileExtensions  []string
    ExcludeDirs     []string
    IncludePatterns []string
}
```

**配置项说明：**

| 配置项 | 类型 | 说明 |
|--------|------|------|
| `FilePatterns` | `[]string` | 文件名 glob 模式白名单，仅匹配文件名部分 |
| `FileExtensions` | `[]string` | 文件扩展名白名单，自动补全 `.` 前缀，不区分大小写 |
| `ExcludeDirs` | `[]string` | 需要排除的目录名列表，匹配目录名或完整路径 |
| `IncludePatterns` | `[]string` | 完整路径 glob 模式白名单，匹配文件完整路径 |

**过滤规则优先级：**
1. 首先检查目录是否在排除列表中（`ExcludeDirs`），如果是则跳过整个目录
2. 然后依次检查扩展名过滤（`FileExtensions`）、文件名模式过滤（`FilePatterns`）、路径包含过滤（`IncludePatterns`）
3. 所有已配置的过滤条件都必须同时满足，文件才会被监听
4. 未配置的过滤条件（空切片）不参与过滤，视为通过

### 3.5 Config - 监听器配置

```go
type Config struct {
    DebounceWindow time.Duration
    PollInterval   time.Duration
    Filters        FilterConfig
}
```

**配置项说明：**

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `DebounceWindow` | `time.Duration` | 100ms | 防抖时间窗口，同一文件同类事件在此窗口内合并为一次 |
| `PollInterval` | `time.Duration` | 50ms | 轮询间隔，监听器多久扫描一次文件系统 |
| `Filters` | `FilterConfig` | 空配置 | 事件过滤规则集合 |

**配置约束与默认值：**
- `DebounceWindow`：必须 >= 0。负数返回 `ErrInvalidConfig`；0 自动使用默认值 100ms
- `PollInterval`：必须 >= 0。负数返回 `ErrInvalidConfig`；0 自动使用默认值 50ms
- 轮询间隔不宜过短（<10ms），否则会占用过多 CPU
- 防抖窗口应大于轮询间隔，否则可能无法有效合并事件

### 3.6 FileWatcher - 文件监听器主体

```go
type FileWatcher struct {
    cfg       Config
    watchedDir string

    mu        sync.Mutex
    running   bool
    stopped   bool
    stopCh    chan struct{}
    wg        sync.WaitGroup

    fileStates     map[string]time.Time
    pendingEvents  map[string]Event
    debounceTimers map[string]*time.Timer

    onCreate EventCallback
    onModify EventCallback
    onDelete EventCallback
}
```

**主要职责：**
- 管理文件监听器的完整生命周期
- 维护文件状态快照（`fileStates`），用于对比检测变更
- 管理防抖定时器（`debounceTimers`）和待触发事件（`pendingEvents`）
- 存储三种事件类型的回调函数
- 保证线程安全，所有公共方法通过互斥锁保护内部状态

**内部字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `cfg` | `Config` | 监听器配置快照，创建后不可变更 |
| `watchedDir` | `string` | 监听的根目录绝对路径 |
| `running` | `bool` | 监听器是否正在运行（后台轮询协程是否活跃） |
| `stopped` | `bool` | 监听器是否已永久停止，Stop 后置为 true，不可逆 |
| `stopCh` | `chan struct{}` | 后台协程停止信号通道 |
| `wg` | `sync.WaitGroup` | 后台协程同步等待组 |
| `fileStates` | `map[string]time.Time` | 文件路径 → 最后修改时间的状态快照 |
| `pendingEvents` | `map[string]Event` | 防抖期间待触发的事件缓存 |
| `debounceTimers` | `map[string]*time.Timer` | 各事件的防抖定时器 |
| `onCreate/onModify/onDelete` | `EventCallback` | 三种事件类型的回调函数 |

### 3.7 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrWatcherStopped` | 监听器已永久停止 | 调用 `Stop()` 后，任何注册回调、设置监听目录的操作都会返回此错误 |
| `ErrInvalidConfig` | 配置无效 | `NewWithConfig()` 传入的 `DebounceWindow` 或 `PollInterval` 为负数时返回 |
| `ErrNilCallback` | 回调函数为空 | 调用 `OnCreate`、`OnModify`、`OnDelete` 时传入 nil 回调函数 |
| `ErrNoWatchedDir` | 未设置监听目录 | 启动监听器时未调用 `Watch()` 设置监听目录 |
| `ErrDirNotExist` | 监听目录不存在 | 调用 `Watch()` 时传入的目录不存在或不是目录 |

## 4. 核心机制详解

### 4.1 监听流程总览

```
New() / NewWithConfig()
       │
       ▼
    Watch(dir)  ──  设置监听目录，执行初始扫描，建立文件状态基线
       │
       ▼
  OnCreate(cb) ─┐
  OnModify(cb) ─┼──  注册事件回调函数（可选，可分别注册）
  OnDelete(cb) ─┘
       │
       ▼
    Start()  ──  启动后台轮询协程
       │
       ▼
  [ 轮询循环 ]
       │
       ├─ 扫描目录树，获取当前所有文件的修改时间
       ├─ 与上一次状态快照对比，检测创建/修改/删除事件
       ├─ 对检测到的事件进行过滤判断
       ├─ 通过过滤的事件进入防抖队列
       └─ 防抖到期后触发回调函数
       │
       ▼
    Stop()  ──  停止后台轮询，清理防抖定时器，永久停止监听器
```

### 4.2 递归目录监听机制

递归目录监听基于 `filepath.Walk` 实现，每次轮询时完整遍历目录树：

```
watchedDir/
├── file1.txt          ← 直接子文件，监听
├── subdir1/           ← 子目录，递归进入
│   ├── file2.go       ← 子目录文件，监听
│   └── nested/        ← 更深层目录，继续递归
│       └── file3.md   ← 深层文件，监听
├── node_modules/      ← 排除目录，整个跳过
│   └── ...            ← 子文件均不监听
└── .git/              ← 排除目录，整个跳过
    └── ...            ← 子文件均不监听
```

**实现要点：**
- 使用 `filepath.Walk` 进行深度优先遍历
- 遇到排除目录时返回 `filepath.SkipDir`，跳过该目录及其所有子目录
- 目录本身不产生事件，只监听文件的变更
- 初始扫描时建立文件状态快照，已有文件不触发创建事件

### 4.3 事件检测原理

事件检测通过比对"当前文件状态"与"上一次状态快照"实现：

```
  上一次快照                    当前扫描
  ┌──────────────────┐         ┌──────────────────┐
  │ fileA: t1        │   ┌────▶│ fileA: t1        │  无变化
  │ fileB: t2        │   │     │ fileB: t3        │  修改（t2≠t3）
  │ fileC: t4        │   │     │ (不存在)         │  删除
  └──────────────────┘   │     │ fileD: t5        │  创建（新增）
                         │     └──────────────────┘
                         │
                         ▼
                    三方对比检测
                    ┌─────────────────────────────────┐
                    │ 创建：当前有 && 之前无            │
                    │ 修改：当前有 && 之前有 && 时间不同 │
                    │ 删除：当前无 && 之前有            │
                    └─────────────────────────────────┘
```

**检测逻辑：**
1. 每次轮询生成当前所有文件的 `path -> modTime` 映射
2. 遍历当前文件，与上一次快照对比：
   - 路径不存在于快照 → 创建事件
   - 路径存在但修改时间不同 → 修改事件
3. 遍历上一次快照，文件不在当前列表中 → 删除事件
4. 更新快照为当前状态，供下次对比

### 4.4 防抖去重机制

防抖机制确保短时间内同一文件的同类事件只触发一次回调：

```
  事件流:  [创建] [创建] [创建]
           t1     t2     t3
             │      │      │
             ▼      ▼      ▼
          ┌──────────────────────┐
          │  防抖窗口 (DebounceWindow) │
          │  每次事件重置计时器        │
          └──────────────────────┘
                              │
                              ▼
                         触发一次回调
```

**防抖流程：**
1. 事件到达时，计算防抖 key（事件类型 + 文件路径）
2. 如果该 key 已有待触发的定时器，先停止旧定时器
3. 将事件存入 `pendingEvents` 缓存
4. 启动新的 `time.AfterFunc` 定时器，延迟 `DebounceWindow`
5. 定时器到期后：
   - 从缓存中取出事件
   - 清理对应的定时器和缓存条目
   - 调用对应的回调函数

**设计特点：**
- 不同类型的事件（创建/修改/删除）独立防抖，互不干扰
- 防抖窗口内多次同类事件只保留最后一次，定时器重置
- 防抖回调在定时器 goroutine 中异步执行
- Stop 时会停止所有未到期的防抖定时器

### 4.5 事件过滤机制

事件过滤采用多层过滤链的方式，所有条件都满足才通过：

```
文件路径 → [目录排除检查] → [扩展名过滤] → [文件名模式过滤] → [路径包含过滤] → 通过
             ↓不通过           ↓不通过          ↓不通过            ↓不通过
             跳过             忽略             忽略              忽略
```

**各过滤层说明：**

| 过滤层 | 检查对象 | 匹配方式 | 空配置行为 |
|--------|----------|----------|------------|
| 目录排除 | 目录路径 | 目录名相等 / 路径前缀匹配 | 不排除任何目录 |
| 扩展名过滤 | 文件扩展名 | 精确匹配（忽略大小写） | 不过滤 |
| 文件名模式 | 文件名（base） | `filepath.Match` glob 匹配 | 不过滤 |
| 路径包含 | 完整文件路径 | `filepath.Match` glob 匹配 | 不过滤 |

### 4.6 生命周期管理

#### 4.6.1 状态流转

```
  初始态 (stopped=false, running=false)
      │
      │ Watch(dir)
      ▼
  已配置 (stopped=false, running=false, watchedDir=xxx)
      │
      │ Start()
      ▼
  运行中 (stopped=false, running=true)
      │
      │ Stop()
      ▼
  已停止 (stopped=true, running=false)  ←  终态，不可逆
```

#### 4.6.2 各状态行为说明

| 状态 | `stopped` | `running` | 行为 |
|------|-----------|-----------|------|
| **初始态** | `false` | `false` | 可设置监听目录、注册回调，但无事件产生 |
| **已配置** | `false` | `false` | 已设置监听目录，可注册回调，可调用 Start 启动 |
| **运行中** | `false` | `true` | 后台轮询活跃，检测并分发文件事件 |
| **已停止** | `true` | `false` | 所有修改操作返回 `ErrWatcherStopped`，不可重启 |

#### 4.6.3 Start 流程

```
Start()
   │
   ├─ mu.Lock()
   ├─ stopped == true → mu.Unlock()，直接返回（已停止无法重启）
   ├─ running == true → mu.Unlock()，直接返回（幂等）
   ├─ watchedDir == "" → mu.Unlock()，直接返回（未设置监听目录）
   ├─ running = true
   ├─ stopCh = make(chan struct{})
   ├─ mu.Unlock()
   │
   ├─ wg.Add(1)
   └─ 启动 pollLoop 后台协程
```

#### 4.6.4 Stop 流程

```
Stop()
   │
   ├─ mu.Lock()
   ├─ stopped == true → mu.Unlock()，直接返回（幂等）
   ├─ stopped = true  ← 永久标记
   ├─ 若 running：
   │     ├─ running = false
   │     └─ close(stopCh)
   ├─ 停止所有防抖定时器
   ├─ 清空防抖定时器 map
   ├─ mu.Unlock()
   │
   └─ wg.Wait()（等待后台协程退出）
```

**不可逆停止约定：**
- `Stop()` 一旦调用，`stopped` 标记永久设置为 `true`，监听器进入终态
- 停止后，`Watch()`、`OnCreate()`、`OnModify()`、`OnDelete()` 均返回 `ErrWatcherStopped`
- `Start()` 在已停止状态下调用会被静默忽略，不会重启
- 如需重新使用，必须创建新的 `FileWatcher` 实例

**资源安全：**
- `Start()` 和 `Stop()` 均支持幂等调用，重复调用无副作用
- `Stop()` 会阻塞直到后台轮询协程完全退出，防止协程泄漏
- `Stop()` 同时会停止所有未到期的防抖定时器，清理相关资源

### 4.7 轮询循环

```
pollLoop（后台协程）
   │
   ├─ 创建 ticker = time.NewTicker(PollInterval)
   │
   └─ [循环]
         │
         ├─ select
         │     ├─ stopCh 关闭 → ticker.Stop()，wg.Done()，退出
         │     └─ ticker.C 触发 → 调用 poll()
         │
         └─ 继续循环
```

**Poll 函数执行流程：**
1. 获取互斥锁
2. 检查是否已停止或未设置监听目录，是则直接返回
3. 调用 `filepath.Walk` 扫描目录树，生成当前文件状态
4. 与上一次状态快照对比，检测创建、修改、删除事件
5. 对每个事件调用 `scheduleDebounced` 进入防抖队列
6. 更新 `fileStates` 为当前状态
7. 释放互斥锁

## 5. 线程安全设计

所有公共方法均通过互斥锁 `mu` 保护内部状态：
- **修改操作**（`Watch`、`OnCreate`、`OnModify`、`OnDelete`、`Start`、`Stop`）：获取排他锁
- **读操作**（`IsRunning`、`WatchedDir`、`WatchedFileCount`）：同样获取排他锁
- 防抖定时器回调中也会获取锁以操作共享状态
- 单元测试中的并发测试验证多 goroutine 并发调用无竞态条件

## 6. 使用示例

### 6.1 基础使用：监听目录文件变更

```go
package main

import (
    "errors"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "solocoder-go/internal/filewatcher"
)

func main() {
    cfg := filewatcher.Config{
        DebounceWindow: 200 * time.Millisecond,
        PollInterval:   100 * time.Millisecond,
        Filters: filewatcher.FilterConfig{
            FileExtensions: []string{".go", ".txt"},
            ExcludeDirs:    []string{"node_modules", ".git"},
        },
    }

    fw, err := filewatcher.NewWithConfig(cfg)
    if err != nil {
        log.Fatalf("创建文件监听器失败: %v", err)
    }

    watchDir := "./src"
    if err := fw.Watch(watchDir); err != nil {
        log.Fatalf("设置监听目录失败: %v", err)
    }

    _ = fw.OnCreate(func(evt filewatcher.Event) {
        fmt.Printf("[创建] %s\n", evt.Path)
    })

    _ = fw.OnModify(func(evt filewatcher.Event) {
        fmt.Printf("[修改] %s\n", evt.Path)
    })

    _ = fw.OnDelete(func(evt filewatcher.Event) {
        fmt.Printf("[删除] %s\n", evt.Path)
    })

    fw.Start()
    defer fw.Stop()

    fmt.Printf("正在监听目录: %s\n", fw.WatchedDir())
    fmt.Println("按 Ctrl+C 退出...")

    stopCh := make(chan os.Signal, 1)
    signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
    <-stopCh

    fmt.Println("正在停止监听器...")
}
```

### 6.2 配置热加载场景

```go
type ConfigManager struct {
    watcher *filewatcher.FileWatcher
    config  *AppConfig
    mu      sync.RWMutex
}

func NewConfigManager(configPath string) (*ConfigManager, error) {
    fw := filewatcher.New()

    cm := &ConfigManager{
        watcher: fw,
    }

    if err := fw.Watch(filepath.Dir(configPath)); err != nil {
        return nil, err
    }

    _ = fw.OnModify(func(evt filewatcher.Event) {
        if filepath.Base(evt.Path) == filepath.Base(configPath) {
            cm.reloadConfig(evt.Path)
        }
    })

    fw.Start()
    return cm, nil
}

func (cm *ConfigManager) reloadConfig(path string) {
    // 重新加载配置文件...
    log.Printf("配置文件已更新: %s", path)
}
```

### 6.3 使用文件名模式过滤

```go
fw, _ := filewatcher.NewWithConfig(filewatcher.Config{
    Filters: filewatcher.FilterConfig{
        FilePatterns: []string{"*.log", "test_*"},
    },
})

_ = fw.Watch("/var/log")
fw.Start()
// 只有 .log 后缀和 test_ 开头的文件会触发事件
```

### 6.4 检测监听器状态

```go
if fw.IsRunning() {
    fmt.Printf("监听器运行中，监听目录: %s\n", fw.WatchedDir())
    fmt.Printf("当前监听文件数: %d\n", fw.WatchedFileCount())
}
```

### 6.5 优雅关闭处理

```go
// 启动监听器
fw.Start()

// 监听关闭信号
go func() {
    <-shutdownCh
    log.Println("正在停止文件监听器...")
    fw.Stop()
    log.Println("文件监听器已停止")
    close(doneCh)
}()

// 使用监听器...

// 等待完全停止
<-doneCh
```

### 6.6 单元测试风格的模拟场景

```go
func TestFileWatcher_CreateEvent(t *testing.T) {
    tmpDir := t.TempDir()

    var createEvents []filewatcher.Event
    var mu sync.Mutex

    fw, _ := filewatcher.NewWithConfig(filewatcher.Config{
        DebounceWindow: 20 * time.Millisecond,
        PollInterval:   10 * time.Millisecond,
    })
    _ = fw.OnCreate(func(evt filewatcher.Event) {
        mu.Lock()
        createEvents = append(createEvents, evt)
        mu.Unlock()
    })

    err := fw.Watch(tmpDir)
    require.NoError(t, err)

    fw.Start()
    defer fw.Stop()

    time.Sleep(50 * time.Millisecond)

    testFile := filepath.Join(tmpDir, "test.txt")
    err = os.WriteFile(testFile, []byte("hello"), 0644)
    require.NoError(t, err)

    time.Sleep(100 * time.Millisecond)

    mu.Lock()
    assert.Len(t, createEvents, 1)
    assert.Equal(t, filewatcher.EventCreate, createEvents[0].Type)
    assert.True(t, strings.HasSuffix(createEvents[0].Path, "test.txt"))
    mu.Unlock()
}
```

## 7. 文件结构

```
internal/filewatcher/
├── filewatcher.go      # 核心实现（监听器、事件、配置、过滤、防抖）
└── filewatcher_test.go # 单元测试（覆盖正常流程、边界条件、异常分支、并发场景）

docs/
└── filewatcher.md      # 本文档
```

## 8. 测试覆盖说明

单元测试覆盖以下场景类别：

| 测试类别 | 代表性测试用例 | 覆盖目标 |
|----------|---------------|----------|
| **基础创建** | `TestNew`、`TestDefaultConfig`、`TestNewWithConfig_*` | 构造函数、默认值、无效配置校验 |
| **目录监听** | `TestWatch_Success`、`TestWatch_NonExistentDir`、`TestWatch_FileInsteadOfDir` | 正常监听、目录不存在、路径是文件 |
| **事件回调** | `TestFileCreateEvent`、`TestFileModifyEvent`、`TestFileDeleteEvent` | 三种事件类型的检测与回调 |
| **递归监听** | `TestRecursiveDirectory` | 子目录、深层嵌套目录的文件监听 |
| **防抖去重** | `TestDebounce_DuplicateEventsMerged`、`TestDebounce_CreateAndModifySameFile` | 短时间多次事件的合并、不同事件类型独立防抖 |
| **过滤规则** | `TestFilter_FileExtensions`、`TestFilter_FilePatterns`、`TestFilter_ExcludeDirs`、`TestFilter_IncludePatterns`、`TestFilter_Combined` | 各种过滤规则及组合使用 |
| **扩展名处理** | `TestFilter_FileExtensions_WithoutDot`、`TestNormalizeExtensions` | 扩展名自动补全点号、大小写归一化 |
| **生命周期** | `TestStartStop_Idempotent`、`TestIsRunning`、`TestStart_WithoutWatch` | 幂等启停、运行状态查询、未配置目录启动 |
| **停止状态** | `TestStop_RejectsCallbacksRegistration`、`TestStop_RejectsWatch`、`TestStart_AfterStop`、`TestStop_WithoutStart` | Stop 后操作受限、不可逆、未启动时停止 |
| **多回调** | `TestMultipleCallbacks` | 三种回调同时注册，各事件独立触发 |
| **并发安全** | `TestConcurrent_FileOperations` | 多 goroutine 并发创建文件，事件检测正确 |
| **状态查询** | `TestWatchedFileCount`、`TestEventStruct` | 文件计数、事件结构体字段 |
| **边界条件** | `TestPassesFilters_NoFilters`、`TestIsExcludedDir_NoExcludes` | 空过滤配置行为 |
| **工具函数** | `TestEventType_String`、`TestDebounceKey` | 辅助函数正确性 |
| **默认值推导** | `TestDebounceWindow_ZeroUsesDefault`、`TestPollInterval_ZeroUsesDefault` | 零值配置自动使用默认值 |
| **初始文件** | `TestWatch_InitialFilesNotTriggerEvents` | 初始已有文件不触发创建事件 |
| **空回调** | `TestOnCreate_NilCallback`、`TestOnModify_NilCallback`、`TestOnDelete_NilCallback` | 注册 nil 回调返回错误 |
