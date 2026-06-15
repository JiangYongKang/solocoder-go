# RBAC 访问控制模块

## 1. 模块概述

RBAC（Role-Based Access Control，基于角色的访问控制）模块提供了一套完整的权限管理系统，支持角色定义、权限资源注册、角色-权限绑定、用户-角色分配以及权限校验决策等核心功能。该模块设计为线程安全，可在并发环境下安全使用。

### 1.1 核心功能

| 功能分类 | 功能描述 |
|---------|---------|
| 角色管理 | 创建、删除、查询角色，角色名称唯一约束 |
| 权限资源管理 | 注册、注销可被访问控制的资源与操作 |
| 角色-权限绑定 | 为角色授予或撤销权限，支持多对多关系 |
| 用户-角色分配 | 为用户分配或撤销角色，支持多对多关系 |
| 权限校验 | 决策引擎判定用户是否拥有指定资源的操作权限 |

## 2. 核心结构体职责

### 2.1 Role

```go
type Role struct {
    ID          string
    Name        string
    Description string
}
```

**职责**：表示系统中的角色实体
- `ID`：角色唯一标识符，不可重复
- `Name`：角色名称，业务唯一标识，不可重复
- `Description`：角色描述信息

### 2.2 Permission

```go
type Permission struct {
    Resource string
    Action   string
}
```

**职责**：表示具体的权限项，由资源和操作组合而成
- `Resource`：资源标识，如 `article`、`user`
- `Action`：操作标识，如 `read`、`write`、`delete`
- 组合形式：`resource:action`，如 `article:read`

### 2.3 RoleWithPermissions

```go
type RoleWithPermissions struct {
    Role
    Permissions []Permission
}
```

**职责**：角色与其拥有的权限集合的组合视图，用于查询时返回完整的角色权限信息

### 2.4 UserWithRoles

```go
type UserWithRoles struct {
    UserID      string
    Roles       []Role
    Permissions []Permission
}
```

**职责**：用户与其所分配角色及聚合权限的完整视图
- `UserID`：用户唯一标识
- `Roles`：用户拥有的所有角色列表
- `Permissions`：用户通过角色聚合后的所有权限集合（已去重）

### 2.5 Decision

```go
type Decision struct {
    Allowed bool
    Reason  string
}
```

**职责**：权限校验决策结果
- `Allowed`：`true` 表示允许访问，`false` 表示拒绝访问
- `Reason`：拒绝时的详细原因说明，允许时为空字符串

### 2.6 RBAC

```go
type RBAC struct {
    mu                sync.RWMutex
    roles             map[string]*Role
    roleByName        map[string]string
    permissions       map[Permission]bool
    rolePermissions   map[string]map[Permission]bool
    userRoles         map[string]map[string]bool
}
```

**职责**：RBAC 引擎核心，管理所有权限相关数据和操作
- 使用 `sync.RWMutex` 保证并发安全
- `roles`：角色 ID 到角色对象的映射
- `roleByName`：角色名称到角色 ID 的映射，用于快速按名称查询
- `permissions`：已注册权限集合
- `rolePermissions`：角色 ID 到其权限集合的映射
- `userRoles`：用户 ID 到其角色集合的映射

## 3. 权限校验决策流程

### 3.1 决策引擎流程

```
CheckPermission(userID, resource, action)
        │
        ▼
  参数合法性校验
        │
        ├─► userID 为空 → 拒绝：ErrEmptyUserID
        ├─► resource 为空 → 拒绝：ErrEmptyResource
        └─► action 为空 → 拒绝：ErrEmptyAction
        │
        ▼
  检查权限是否已注册
        │
        └─► 未注册 → 拒绝："permission xxx is not registered"
        │
        ▼
  查询用户角色
        │
        └─► 用户无角色 → 拒绝："user xxx has no roles assigned"
        │
        ▼
  遍历用户所有角色
        │
        └─► 任一角色拥有该权限 → 允许：Allowed = true
        │
        ▼
  所有角色均无该权限
        │
        └─► 拒绝："user xxx does not have permission xxx"
```

### 3.2 决策规则

1. **默认拒绝**：除非明确授权，否则默认拒绝访问
2. **权限聚合**：用户的权限是其所有角色权限的并集
3. **权限去重**：用户从多个角色获得相同权限时，只保留一份
4. **拒绝原因**：拒绝访问时必须提供清晰的原因说明

## 4. API 接口说明

### 4.1 角色管理

| 方法 | 签名 | 说明 |
|------|------|------|
| 创建角色 | `CreateRole(roleID, name, description string) (*Role, error)` | 创建新角色，ID 和名称均不可重复 |
| 删除角色 | `DeleteRole(roleID string) error` | 删除角色，角色被用户使用时不可删除 |
| 查询角色 | `GetRole(roleID string) (*Role, error)` | 按 ID 查询角色 |
| 按名称查询 | `GetRoleByName(name string) (*Role, error)` | 按名称查询角色 |
| 角色列表 | `ListRoles() []Role` | 获取所有角色，按 ID 排序 |
| 角色权限 | `GetRoleWithPermissions(roleID string) (*RoleWithPermissions, error)` | 获取角色及其权限 |

### 4.2 权限资源管理

| 方法 | 签名 | 说明 |
|------|------|------|
| 注册权限 | `RegisterPermission(resource, action string) (Permission, error)` | 注册新的权限项 |
| 注销权限 | `UnregisterPermission(resource, action string) error` | 注销权限，被角色使用时不可注销 |
| 权限存在 | `HasPermission(resource, action string) bool` | 检查权限是否已注册 |
| 权限列表 | `ListPermissions() []Permission` | 获取所有权限，按资源+操作排序 |

### 4.3 角色-权限绑定

| 方法 | 签名 | 说明 |
|------|------|------|
| 授予权限 | `GrantPermission(roleID, resource, action string) error` | 为角色授予权限 |
| 撤销权限 | `RevokePermission(roleID, resource, action string) error` | 撤销角色的权限 |
| 查询权限 | `GetRolePermissions(roleID string) ([]Permission, error)` | 获取角色的所有权限 |

### 4.4 用户-角色分配

| 方法 | 签名 | 说明 |
|------|------|------|
| 分配角色 | `AssignRole(userID, roleID string) error` | 为用户分配角色 |
| 撤销角色 | `RevokeRole(userID, roleID string) error` | 撤销用户的角色 |
| 查询用户角色 | `GetUserRoles(userID string) ([]Role, error)` | 获取用户的所有角色 |
| 查询用户权限 | `GetUserPermissions(userID string) ([]Permission, error)` | 获取用户的聚合权限 |
| 用户完整信息 | `GetUserWithRoles(userID string) (*UserWithRoles, error)` | 获取用户的角色和权限 |

### 4.5 权限校验

| 方法 | 签名 | 说明 |
|------|------|------|
| 权限校验 | `CheckPermission(userID, resource, action string) Decision` | 校验用户是否有权限执行操作 |

### 4.6 辅助方法

| 方法 | 签名 | 说明 |
|------|------|------|
| 角色数量 | `RoleCount() int` | 获取角色总数 |
| 权限数量 | `PermissionCount() int` | 获取权限总数 |
| 用户数量 | `UserCount() int` | 获取有角色分配的用户数 |

## 5. 使用示例

### 5.1 基础使用流程

```go
package main

import (
    "fmt"
    "solocoder-go/internal/rbac"
)

func main() {
    // 1. 创建 RBAC 引擎
    engine := rbac.NewRBAC()

    // 2. 创建角色
    admin, _ := engine.CreateRole("admin", "系统管理员", "拥有所有系统权限")
    editor, _ := engine.CreateRole("editor", "内容编辑", "可以管理文章内容")
    viewer, _ := engine.CreateRole("viewer", "访客", "仅可查看内容")

    // 3. 注册权限资源
    engine.RegisterPermission("article", "read")
    engine.RegisterPermission("article", "write")
    engine.RegisterPermission("article", "delete")
    engine.RegisterPermission("user", "read")
    engine.RegisterPermission("user", "write")

    // 4. 为角色授予权限
    engine.GrantPermission("admin", "article", "read")
    engine.GrantPermission("admin", "article", "write")
    engine.GrantPermission("admin", "article", "delete")
    engine.GrantPermission("admin", "user", "read")
    engine.GrantPermission("admin", "user", "write")

    engine.GrantPermission("editor", "article", "read")
    engine.GrantPermission("editor", "article", "write")

    engine.GrantPermission("viewer", "article", "read")

    // 5. 为用户分配角色
    engine.AssignRole("alice", "admin")
    engine.AssignRole("bob", "editor")
    engine.AssignRole("charlie", "viewer")

    // 6. 权限校验
    decision := engine.CheckPermission("alice", "article", "delete")
    if decision.Allowed {
        fmt.Println("alice 可以删除文章")
    } else {
        fmt.Printf("拒绝原因: %s\n", decision.Reason)
    }

    decision = engine.CheckPermission("bob", "user", "read")
    if !decision.Allowed {
        fmt.Printf("bob 不能读取用户信息: %s\n", decision.Reason)
    }
}
```

### 5.2 查询用户完整信息

```go
// 获取用户的所有角色和聚合权限
userInfo, err := engine.GetUserWithRoles("alice")
if err != nil {
    // 处理错误
}

fmt.Printf("用户 %s 的角色:\n", userInfo.UserID)
for _, role := range userInfo.Roles {
    fmt.Printf("  - %s (%s)\n", role.Name, role.Description)
}

fmt.Println("用户拥有的权限:")
for _, perm := range userInfo.Permissions {
    fmt.Printf("  - %s:%s\n", perm.Resource, perm.Action)
}
```

### 5.3 查询角色权限

```go
// 获取角色及其权限
roleInfo, err := engine.GetRoleWithPermissions("admin")
if err != nil {
    // 处理错误
}

fmt.Printf("角色 %s 拥有的权限:\n", roleInfo.Name)
for _, perm := range roleInfo.Permissions {
    fmt.Printf("  - %s:%s\n", perm.Resource, perm.Action)
}
```

### 5.4 错误处理示例

```go
// 尝试创建重复角色
_, err := engine.CreateRole("admin", "重复管理员", "desc")
if err != nil {
    if errors.Is(err, rbac.ErrRoleExists) {
        fmt.Println("角色已存在")
    }
}

// 尝试删除被使用的角色
err = engine.DeleteRole("admin")
if err != nil {
    if errors.Is(err, rbac.ErrRoleInUse) {
        fmt.Println("角色正在被使用，无法删除")
    }
}

// 尝试授权未注册的权限
err = engine.GrantPermission("admin", "unknown", "action")
if err != nil {
    if errors.Is(err, rbac.ErrPermissionNotFound) {
        fmt.Println("权限未注册")
    }
}
```

## 6. 错误定义

| 错误变量 | 错误信息 | 触发场景 |
|---------|---------|---------|
| `ErrRoleNotFound` | role not found | 角色不存在 |
| `ErrRoleExists` | role already exists | 创建重复角色（ID 或名称重复） |
| `ErrPermissionNotFound` | permission not found | 权限不存在 |
| `ErrPermissionExists` | permission already exists | 注册重复权限 |
| `ErrUserNotFound` | user not found | 用户不存在 |
| `ErrEmptyRoleName` | role name cannot be empty | 角色名称为空 |
| `ErrEmptyRoleID` | role id cannot be empty | 角色 ID 为空 |
| `ErrEmptyResource` | resource cannot be empty | 资源标识为空 |
| `ErrEmptyAction` | action cannot be empty | 操作标识为空 |
| `ErrEmptyUserID` | user id cannot be empty | 用户 ID 为空 |
| `ErrRoleInUse` | role is in use and cannot be deleted | 删除被用户使用的角色 |
| `ErrPermissionInUse` | permission is in use and cannot be unregistered | 注销被角色使用的权限 |

## 7. 线程安全

本模块所有公共方法均通过 `sync.RWMutex` 保证线程安全：
- 读操作（查询类方法）使用读锁 `RLock()`，支持并发读
- 写操作（修改类方法）使用写锁 `Lock()`，保证原子性

可在高并发场景下安全使用，无需额外加锁。

## 8. 设计要点

1. **数据隔离**：角色、权限、用户数据分别存储，通过映射关系关联
2. **索引优化**：`roleByName` 索引支持快速按角色名称查询
3. **防御性拷贝**：返回 Role 对象时返回副本，防止外部修改内部状态
4. **排序保证**：列表查询结果按指定规则排序，确保结果可预测
5. **权限去重**：用户权限聚合时自动去重，避免重复权限项
