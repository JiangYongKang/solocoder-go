package rbac

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNewRBAC(t *testing.T) {
	rbac := NewRBAC()
	if rbac == nil {
		t.Fatal("NewRBAC returned nil")
	}
	if rbac.RoleCount() != 0 {
		t.Errorf("expected 0 roles, got %d", rbac.RoleCount())
	}
	if rbac.PermissionCount() != 0 {
		t.Errorf("expected 0 permissions, got %d", rbac.PermissionCount())
	}
	if rbac.UserCount() != 0 {
		t.Errorf("expected 0 users, got %d", rbac.UserCount())
	}
}

func TestCreateRole(t *testing.T) {
	rbac := NewRBAC()

	role, err := rbac.CreateRole("admin", "管理员", "系统管理员角色")
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	if role.ID != "admin" {
		t.Errorf("expected role ID 'admin', got '%s'", role.ID)
	}
	if role.Name != "管理员" {
		t.Errorf("expected role name '管理员', got '%s'", role.Name)
	}
	if role.Description != "系统管理员角色" {
		t.Errorf("expected description mismatch")
	}
	if rbac.RoleCount() != 1 {
		t.Errorf("expected 1 role, got %d", rbac.RoleCount())
	}
}

func TestCreateRoleEmptyID(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.CreateRole("", "admin", "desc")
	if !errors.Is(err, ErrEmptyRoleID) {
		t.Errorf("expected ErrEmptyRoleID, got %v", err)
	}
}

func TestCreateRoleEmptyName(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.CreateRole("admin", "", "desc")
	if !errors.Is(err, ErrEmptyRoleName) {
		t.Errorf("expected ErrEmptyRoleName, got %v", err)
	}
}

func TestCreateRoleDuplicateID(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.CreateRole("admin", "管理员", "desc")
	if err != nil {
		t.Fatal(err)
	}

	_, err = rbac.CreateRole("admin", "超级管理员", "desc")
	if !errors.Is(err, ErrRoleExists) {
		t.Errorf("expected ErrRoleExists for duplicate ID, got %v", err)
	}
}

func TestCreateRoleDuplicateName(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.CreateRole("admin", "管理员", "desc")
	if err != nil {
		t.Fatal(err)
	}

	_, err = rbac.CreateRole("superadmin", "管理员", "desc")
	if !errors.Is(err, ErrRoleExists) {
		t.Errorf("expected ErrRoleExists for duplicate name, got %v", err)
	}
}

func TestDeleteRole(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	if rbac.RoleCount() != 1 {
		t.Fatalf("expected 1 role, got %d", rbac.RoleCount())
	}

	err := rbac.DeleteRole("admin")
	if err != nil {
		t.Fatalf("DeleteRole failed: %v", err)
	}
	if rbac.RoleCount() != 0 {
		t.Errorf("expected 0 roles after delete, got %d", rbac.RoleCount())
	}
}

func TestDeleteRoleNotFound(t *testing.T) {
	rbac := NewRBAC()

	err := rbac.DeleteRole("nonexistent")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestDeleteRoleEmptyID(t *testing.T) {
	rbac := NewRBAC()

	err := rbac.DeleteRole("")
	if !errors.Is(err, ErrEmptyRoleID) {
		t.Errorf("expected ErrEmptyRoleID, got %v", err)
	}
}

func TestDeleteRoleInUse(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.AssignRole("user1", "admin")

	err := rbac.DeleteRole("admin")
	if !errors.Is(err, ErrRoleInUse) {
		t.Errorf("expected ErrRoleInUse, got %v", err)
	}
}

func TestGetRole(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "系统管理员")

	role, err := rbac.GetRole("admin")
	if err != nil {
		t.Fatalf("GetRole failed: %v", err)
	}
	if role.ID != "admin" {
		t.Errorf("expected ID 'admin', got '%s'", role.ID)
	}
	if role.Name != "管理员" {
		t.Errorf("expected name '管理员', got '%s'", role.Name)
	}
	if role.Description != "系统管理员" {
		t.Errorf("expected description '系统管理员', got '%s'", role.Description)
	}
}

func TestGetRoleNotFound(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.GetRole("nonexistent")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestGetRoleEmptyID(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.GetRole("")
	if !errors.Is(err, ErrEmptyRoleID) {
		t.Errorf("expected ErrEmptyRoleID, got %v", err)
	}
}

func TestGetRoleByName(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")

	role, err := rbac.GetRoleByName("管理员")
	if err != nil {
		t.Fatalf("GetRoleByName failed: %v", err)
	}
	if role.ID != "admin" {
		t.Errorf("expected ID 'admin', got '%s'", role.ID)
	}
}

func TestGetRoleByNameNotFound(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.GetRoleByName("不存在")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestListRoles(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.CreateRole("editor", "编辑", "desc")
	rbac.CreateRole("viewer", "查看者", "desc")

	roles := rbac.ListRoles()
	if len(roles) != 3 {
		t.Fatalf("expected 3 roles, got %d", len(roles))
	}

	expectedIDs := []string{"admin", "editor", "viewer"}
	for i, role := range roles {
		if role.ID != expectedIDs[i] {
			t.Errorf("expected role[%d].ID = %s, got %s", i, expectedIDs[i], role.ID)
		}
	}
}

func TestListRolesEmpty(t *testing.T) {
	rbac := NewRBAC()

	roles := rbac.ListRoles()
	if len(roles) != 0 {
		t.Errorf("expected 0 roles, got %d", len(roles))
	}
}

func TestRegisterPermission(t *testing.T) {
	rbac := NewRBAC()

	perm, err := rbac.RegisterPermission("article", "read")
	if err != nil {
		t.Fatalf("RegisterPermission failed: %v", err)
	}
	if perm.Resource != "article" {
		t.Errorf("expected resource 'article', got '%s'", perm.Resource)
	}
	if perm.Action != "read" {
		t.Errorf("expected action 'read', got '%s'", perm.Action)
	}
	if rbac.PermissionCount() != 1 {
		t.Errorf("expected 1 permission, got %d", rbac.PermissionCount())
	}
}

func TestRegisterPermissionEmptyResource(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.RegisterPermission("", "read")
	if !errors.Is(err, ErrEmptyResource) {
		t.Errorf("expected ErrEmptyResource, got %v", err)
	}
}

func TestRegisterPermissionEmptyAction(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.RegisterPermission("article", "")
	if !errors.Is(err, ErrEmptyAction) {
		t.Errorf("expected ErrEmptyAction, got %v", err)
	}
}

func TestRegisterPermissionDuplicate(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.RegisterPermission("article", "read")
	if err != nil {
		t.Fatal(err)
	}

	_, err = rbac.RegisterPermission("article", "read")
	if !errors.Is(err, ErrPermissionExists) {
		t.Errorf("expected ErrPermissionExists, got %v", err)
	}
}

func TestUnregisterPermission(t *testing.T) {
	rbac := NewRBAC()

	rbac.RegisterPermission("article", "read")
	if rbac.PermissionCount() != 1 {
		t.Fatalf("expected 1 permission, got %d", rbac.PermissionCount())
	}

	err := rbac.UnregisterPermission("article", "read")
	if err != nil {
		t.Fatalf("UnregisterPermission failed: %v", err)
	}
	if rbac.PermissionCount() != 0 {
		t.Errorf("expected 0 permissions after unregister, got %d", rbac.PermissionCount())
	}
}

func TestUnregisterPermissionNotFound(t *testing.T) {
	rbac := NewRBAC()

	err := rbac.UnregisterPermission("article", "read")
	if !errors.Is(err, ErrPermissionNotFound) {
		t.Errorf("expected ErrPermissionNotFound, got %v", err)
	}
}

func TestUnregisterPermissionInUse(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.GrantPermission("admin", "article", "read")

	err := rbac.UnregisterPermission("article", "read")
	if !errors.Is(err, ErrPermissionInUse) {
		t.Errorf("expected ErrPermissionInUse, got %v", err)
	}
}

func TestHasPermission(t *testing.T) {
	rbac := NewRBAC()

	rbac.RegisterPermission("article", "read")

	if !rbac.HasPermission("article", "read") {
		t.Error("expected HasPermission to return true")
	}
	if rbac.HasPermission("article", "write") {
		t.Error("expected HasPermission to return false for unregistered permission")
	}
}

func TestListPermissions(t *testing.T) {
	rbac := NewRBAC()

	rbac.RegisterPermission("article", "read")
	rbac.RegisterPermission("article", "write")
	rbac.RegisterPermission("user", "read")

	perms := rbac.ListPermissions()
	if len(perms) != 3 {
		t.Fatalf("expected 3 permissions, got %d", len(perms))
	}

	if perms[0].Resource != "article" || perms[0].Action != "read" {
		t.Errorf("expected first permission article:read")
	}
	if perms[1].Resource != "article" || perms[1].Action != "write" {
		t.Errorf("expected second permission article:write")
	}
	if perms[2].Resource != "user" || perms[2].Action != "read" {
		t.Errorf("expected third permission user:read")
	}
}

func TestPermissionString(t *testing.T) {
	perm := Permission{Resource: "article", Action: "read"}
	if perm.String() != "article:read" {
		t.Errorf("expected 'article:read', got '%s'", perm.String())
	}
}

func TestParsePermission(t *testing.T) {
	perm, err := ParsePermission("article:read")
	if err != nil {
		t.Fatalf("ParsePermission failed: %v", err)
	}
	if perm.Resource != "article" {
		t.Errorf("expected resource 'article', got '%s'", perm.Resource)
	}
	if perm.Action != "read" {
		t.Errorf("expected action 'read', got '%s'", perm.Action)
	}
}

func TestParsePermissionInvalid(t *testing.T) {
	_, err := ParsePermission("invalid")
	if err == nil {
		t.Error("expected error for invalid permission format")
	}
}

func TestGrantPermission(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")

	err := rbac.GrantPermission("admin", "article", "read")
	if err != nil {
		t.Fatalf("GrantPermission failed: %v", err)
	}

	perms, err := rbac.GetRolePermissions("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}
}

func TestGrantPermissionRoleNotFound(t *testing.T) {
	rbac := NewRBAC()

	rbac.RegisterPermission("article", "read")

	err := rbac.GrantPermission("nonexistent", "article", "read")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestGrantPermissionNotFound(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")

	err := rbac.GrantPermission("admin", "article", "read")
	if !errors.Is(err, ErrPermissionNotFound) {
		t.Errorf("expected ErrPermissionNotFound, got %v", err)
	}
}

func TestRevokePermission(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.GrantPermission("admin", "article", "read")

	err := rbac.RevokePermission("admin", "article", "read")
	if err != nil {
		t.Fatalf("RevokePermission failed: %v", err)
	}

	perms, err := rbac.GetRolePermissions("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(perms) != 0 {
		t.Errorf("expected 0 permissions after revoke, got %d", len(perms))
	}
}

func TestRevokePermissionRoleNotFound(t *testing.T) {
	rbac := NewRBAC()

	err := rbac.RevokePermission("nonexistent", "article", "read")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestRevokePermissionNotFound(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")

	err := rbac.RevokePermission("admin", "article", "read")
	if !errors.Is(err, ErrPermissionNotFound) {
		t.Errorf("expected ErrPermissionNotFound, got %v", err)
	}
}

func TestGetRolePermissions(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.RegisterPermission("article", "write")
	rbac.RegisterPermission("user", "read")

	rbac.GrantPermission("admin", "article", "read")
	rbac.GrantPermission("admin", "article", "write")

	perms, err := rbac.GetRolePermissions("admin")
	if err != nil {
		t.Fatalf("GetRolePermissions failed: %v", err)
	}
	if len(perms) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(perms))
	}
}

func TestGetRoleWithPermissions(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "系统管理员")
	rbac.RegisterPermission("article", "read")
	rbac.GrantPermission("admin", "article", "read")

	rp, err := rbac.GetRoleWithPermissions("admin")
	if err != nil {
		t.Fatalf("GetRoleWithPermissions failed: %v", err)
	}
	if rp.ID != "admin" {
		t.Errorf("expected role ID 'admin', got '%s'", rp.ID)
	}
	if rp.Name != "管理员" {
		t.Errorf("expected role name '管理员', got '%s'", rp.Name)
	}
	if len(rp.Permissions) != 1 {
		t.Errorf("expected 1 permission, got %d", len(rp.Permissions))
	}
}

func TestGetRoleWithPermissionsNotFound(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.GetRoleWithPermissions("nonexistent")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestAssignRole(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")

	err := rbac.AssignRole("user1", "admin")
	if err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	if rbac.UserCount() != 1 {
		t.Errorf("expected 1 user, got %d", rbac.UserCount())
	}

	roles, err := rbac.GetUserRoles("user1")
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 {
		t.Errorf("expected 1 role for user1, got %d", len(roles))
	}
}

func TestAssignRoleEmptyUserID(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	err := rbac.AssignRole("", "admin")
	if !errors.Is(err, ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestAssignRoleEmptyRoleID(t *testing.T) {
	rbac := NewRBAC()

	err := rbac.AssignRole("user1", "")
	if !errors.Is(err, ErrEmptyRoleID) {
		t.Errorf("expected ErrEmptyRoleID, got %v", err)
	}
}

func TestAssignRoleNotFound(t *testing.T) {
	rbac := NewRBAC()

	err := rbac.AssignRole("user1", "nonexistent")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestRevokeRole(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.AssignRole("user1", "admin")
	if rbac.UserCount() != 1 {
		t.Fatalf("expected 1 user, got %d", rbac.UserCount())
	}

	err := rbac.RevokeRole("user1", "admin")
	if err != nil {
		t.Fatalf("RevokeRole failed: %v", err)
	}

	if rbac.UserCount() != 0 {
		t.Errorf("expected 0 users after revoke, got %d", rbac.UserCount())
	}

	roles, err := rbac.GetUserRoles("user1")
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 0 {
		t.Errorf("expected 0 roles after revoke, got %d", len(roles))
	}
}

func TestRevokeRoleNotFound(t *testing.T) {
	rbac := NewRBAC()

	err := rbac.RevokeRole("user1", "nonexistent")
	if !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestGetUserRoles(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.CreateRole("editor", "编辑", "desc")
	rbac.AssignRole("user1", "admin")
	rbac.AssignRole("user1", "editor")

	roles, err := rbac.GetUserRoles("user1")
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
	if roles[0].ID != "admin" {
		t.Errorf("expected first role 'admin', got '%s'", roles[0].ID)
	}
	if roles[1].ID != "editor" {
		t.Errorf("expected second role 'editor', got '%s'", roles[1].ID)
	}
}

func TestGetUserRolesNoRoles(t *testing.T) {
	rbac := NewRBAC()

	roles, err := rbac.GetUserRoles("user1")
	if err != nil {
		t.Fatalf("GetUserRoles failed: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("expected 0 roles, got %d", len(roles))
	}
}

func TestGetUserPermissions(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.CreateRole("editor", "编辑", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.RegisterPermission("article", "write")
	rbac.RegisterPermission("article", "delete")
	rbac.RegisterPermission("user", "read")

	rbac.GrantPermission("admin", "article", "read")
	rbac.GrantPermission("admin", "article", "write")
	rbac.GrantPermission("editor", "article", "read")
	rbac.GrantPermission("editor", "user", "read")

	rbac.AssignRole("user1", "admin")
	rbac.AssignRole("user1", "editor")

	perms, err := rbac.GetUserPermissions("user1")
	if err != nil {
		t.Fatalf("GetUserPermissions failed: %v", err)
	}
	if len(perms) != 3 {
		t.Fatalf("expected 3 unique permissions, got %d", len(perms))
	}
}

func TestGetUserPermissionsNoRoles(t *testing.T) {
	rbac := NewRBAC()

	perms, err := rbac.GetUserPermissions("user1")
	if err != nil {
		t.Fatalf("GetUserPermissions failed: %v", err)
	}
	if len(perms) != 0 {
		t.Errorf("expected 0 permissions, got %d", len(perms))
	}
}

func TestGetUserWithRoles(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.GrantPermission("admin", "article", "read")
	rbac.AssignRole("user1", "admin")

	ur, err := rbac.GetUserWithRoles("user1")
	if err != nil {
		t.Fatalf("GetUserWithRoles failed: %v", err)
	}
	if ur.UserID != "user1" {
		t.Errorf("expected UserID 'user1', got '%s'", ur.UserID)
	}
	if len(ur.Roles) != 1 {
		t.Errorf("expected 1 role, got %d", len(ur.Roles))
	}
	if len(ur.Permissions) != 1 {
		t.Errorf("expected 1 permission, got %d", len(ur.Permissions))
	}
}

func TestGetUserWithRolesEmptyUserID(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.GetUserWithRoles("")
	if !errors.Is(err, ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestCheckPermissionAllowed(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.GrantPermission("admin", "article", "read")
	rbac.AssignRole("user1", "admin")

	decision := rbac.CheckPermission("user1", "article", "read")
	if !decision.Allowed {
		t.Errorf("expected permission to be allowed, got denied: %s", decision.Reason)
	}
}

func TestCheckPermissionDeniedNoRoles(t *testing.T) {
	rbac := NewRBAC()

	rbac.RegisterPermission("article", "read")

	decision := rbac.CheckPermission("user1", "article", "read")
	if decision.Allowed {
		t.Error("expected permission to be denied for user with no roles")
	}
	if decision.Reason == "" {
		t.Error("expected denial reason should not be empty")
	}
}

func TestCheckPermissionDeniedNoPermission(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.RegisterPermission("article", "write")
	rbac.GrantPermission("admin", "article", "read")
	rbac.AssignRole("user1", "admin")

	decision := rbac.CheckPermission("user1", "article", "write")
	if decision.Allowed {
		t.Error("expected permission to be denied")
	}
}

func TestCheckPermissionDeniedNotRegistered(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.AssignRole("user1", "admin")

	decision := rbac.CheckPermission("user1", "article", "read")
	if decision.Allowed {
		t.Error("expected permission to be denied for unregistered permission")
	}
}

func TestCheckPermissionEmptyUserID(t *testing.T) {
	rbac := NewRBAC()

	decision := rbac.CheckPermission("", "article", "read")
	if decision.Allowed {
		t.Error("expected permission to be denied for empty user ID")
	}
}

func TestCheckPermissionEmptyResource(t *testing.T) {
	rbac := NewRBAC()

	decision := rbac.CheckPermission("user1", "", "read")
	if decision.Allowed {
		t.Error("expected permission to be denied for empty resource")
	}
}

func TestCheckPermissionEmptyAction(t *testing.T) {
	rbac := NewRBAC()

	decision := rbac.CheckPermission("user1", "article", "")
	if decision.Allowed {
		t.Error("expected permission to be denied for empty action")
	}
}

func TestCheckPermissionMultipleRoles(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("reader", "读者", "desc")
	rbac.CreateRole("writer", "作者", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.RegisterPermission("article", "write")

	rbac.GrantPermission("reader", "article", "read")
	rbac.GrantPermission("writer", "article", "write")

	rbac.AssignRole("user1", "reader")

	decision := rbac.CheckPermission("user1", "article", "read")
	if !decision.Allowed {
		t.Errorf("expected read permission to be allowed")
	}

	decision = rbac.CheckPermission("user1", "article", "write")
	if decision.Allowed {
		t.Error("expected write permission to be denied")
	}

	rbac.AssignRole("user1", "writer")

	decision = rbac.CheckPermission("user1", "article", "write")
	if !decision.Allowed {
		t.Errorf("expected write permission to be allowed after assigning writer role")
	}
}

func TestCheckPermissionMultipleRolesDenyReason(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("role1", "角色1", "desc")
	rbac.CreateRole("role2", "角色2", "desc")
	rbac.RegisterPermission("secret", "access")

	rbac.GrantPermission("role1", "secret", "access")

	rbac.AssignRole("user1", "role2")

	decision := rbac.CheckPermission("user1", "secret", "access")
	if decision.Allowed {
		t.Error("expected permission to be denied")
	}
	if decision.Reason == "" {
		t.Error("expected a denial reason")
	}
}

func TestFullWorkflow(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.CreateRole("admin", "系统管理员", "拥有所有权限")
	if err != nil {
		t.Fatal(err)
	}

	_, err = rbac.CreateRole("editor", "内容编辑", "可以管理文章")
	if err != nil {
		t.Fatal(err)
	}

	_, err = rbac.CreateRole("viewer", "访客", "仅可查看内容")
	if err != nil {
		t.Fatal(err)
	}

	permissions := []struct {
		resource string
		action   string
	}{
		{"article", "read"},
		{"article", "write"},
		{"article", "delete"},
		{"user", "read"},
		{"user", "write"},
		{"user", "delete"},
	}

	for _, p := range permissions {
		_, err := rbac.RegisterPermission(p.resource, p.action)
		if err != nil {
			t.Fatal(err)
		}
	}

	adminPerms := []struct {
		resource string
		action   string
	}{
		{"article", "read"},
		{"article", "write"},
		{"article", "delete"},
		{"user", "read"},
		{"user", "write"},
		{"user", "delete"},
	}
	for _, p := range adminPerms {
		err := rbac.GrantPermission("admin", p.resource, p.action)
		if err != nil {
			t.Fatal(err)
		}
	}

	editorPerms := []struct {
		resource string
		action   string
	}{
		{"article", "read"},
		{"article", "write"},
	}
	for _, p := range editorPerms {
		err := rbac.GrantPermission("editor", p.resource, p.action)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = rbac.GrantPermission("viewer", "article", "read")
	if err != nil {
		t.Fatal(err)
	}

	err = rbac.AssignRole("alice", "admin")
	if err != nil {
		t.Fatal(err)
	}

	err = rbac.AssignRole("bob", "editor")
	if err != nil {
		t.Fatal(err)
	}

	err = rbac.AssignRole("charlie", "viewer")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		user     string
		resource string
		action   string
		allowed  bool
	}{
		{"alice", "article", "read", true},
		{"alice", "article", "write", true},
		{"alice", "article", "delete", true},
		{"alice", "user", "read", true},
		{"alice", "user", "write", true},
		{"alice", "user", "delete", true},
		{"bob", "article", "read", true},
		{"bob", "article", "write", true},
		{"bob", "article", "delete", false},
		{"bob", "user", "read", false},
		{"charlie", "article", "read", true},
		{"charlie", "article", "write", false},
		{"charlie", "article", "delete", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s-%s-%s", tt.user, tt.resource, tt.action), func(t *testing.T) {
			decision := rbac.CheckPermission(tt.user, tt.resource, tt.action)
			if decision.Allowed != tt.allowed {
				t.Errorf("CheckPermission(%s, %s, %s) = %v, want %v. Reason: %s",
					tt.user, tt.resource, tt.action, decision.Allowed, tt.allowed, decision.Reason)
			}
		})
	}

	aliceData, err := rbac.GetUserWithRoles("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(aliceData.Roles) != 1 {
		t.Errorf("expected alice to have 1 role, got %d", len(aliceData.Roles))
	}
	if len(aliceData.Permissions) != 6 {
		t.Errorf("expected alice to have 6 permissions, got %d", len(aliceData.Permissions))
	}

	adminData, err := rbac.GetRoleWithPermissions("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(adminData.Permissions) != 6 {
		t.Errorf("expected admin role to have 6 permissions, got %d", len(adminData.Permissions))
	}
}

func TestConcurrentOperations(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("role1", "角色1", "desc")
	rbac.CreateRole("role2", "角色2", "desc")
	rbac.RegisterPermission("res1", "act1")
	rbac.RegisterPermission("res2", "act2")
	rbac.GrantPermission("role1", "res1", "act1")
	rbac.GrantPermission("role2", "res2", "act2")

	var wg sync.WaitGroup
	var errors int32

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			userID := fmt.Sprintf("user%d", i%10)
			if err := rbac.AssignRole(userID, "role1"); err != nil {
				atomic.AddInt32(&errors, 1)
			}
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			userID := fmt.Sprintf("user%d", i%10)
			rbac.CheckPermission(userID, "res1", "act1")
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rbac.ListRoles()
			rbac.ListPermissions()
		}(i)
	}

	wg.Wait()

	if errors != 0 {
		t.Errorf("expected 0 errors, got %d", errors)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("role1", "角色1", "desc")
	rbac.RegisterPermission("res", "act")
	rbac.GrantPermission("role1", "res", "act")

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					rbac.AssignRole("user1", "role1")
					rbac.RevokeRole("user1", "role1")
				}
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					rbac.CheckPermission("user1", "res", "act")
				}
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					rbac.GetUserRoles("user1")
					rbac.GetUserPermissions("user1")
				}
			}
		}()
	}

	close(stop)
	wg.Wait()
}

func TestEdgeCases(t *testing.T) {
	rbac := NewRBAC()

	_, err := rbac.CreateRole("r1", "Role One", "desc with spaces")
	if err != nil {
		t.Fatal(err)
	}

	_, err = rbac.CreateRole("r2", "Role Two", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = rbac.RegisterPermission("resource-with-dashes", "action_with_underscores")
	if err != nil {
		t.Fatal(err)
	}

	_, err = rbac.RegisterPermission("resource.with.dots", "ACTION.UPPERCASE")
	if err != nil {
		t.Fatal(err)
	}

	err = rbac.GrantPermission("r1", "resource-with-dashes", "action_with_underscores")
	if err != nil {
		t.Fatal(err)
	}

	err = rbac.AssignRole("user-with-special-chars@example.com", "r1")
	if err != nil {
		t.Fatal(err)
	}

	decision := rbac.CheckPermission("user-with-special-chars@example.com", "resource-with-dashes", "action_with_underscores")
	if !decision.Allowed {
		t.Errorf("expected permission to be allowed for special chars, got: %s", decision.Reason)
	}

	decision = rbac.CheckPermission("user-with-special-chars@example.com", "resource.with.dots", "ACTION.UPPERCASE")
	if decision.Allowed {
		t.Error("expected permission to be denied")
	}
}

func TestRevokeRoleMultiple(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.CreateRole("editor", "编辑", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.GrantPermission("admin", "article", "read")
	rbac.GrantPermission("editor", "article", "read")

	rbac.AssignRole("user1", "admin")
	rbac.AssignRole("user1", "editor")

	perms, _ := rbac.GetUserPermissions("user1")
	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}

	err := rbac.RevokeRole("user1", "admin")
	if err != nil {
		t.Fatalf("RevokeRole failed: %v", err)
	}

	perms, _ = rbac.GetUserPermissions("user1")
	if len(perms) != 1 {
		t.Errorf("expected still 1 permission after revoking one role, got %d", len(perms))
	}

	roles, _ := rbac.GetUserRoles("user1")
	if len(roles) != 1 {
		t.Errorf("expected 1 role, got %d", len(roles))
	}
	if roles[0].ID != "editor" {
		t.Errorf("expected remaining role to be 'editor', got '%s'", roles[0].ID)
	}
}

func TestGrantDuplicatePermission(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")

	err := rbac.GrantPermission("admin", "article", "read")
	if err != nil {
		t.Fatal(err)
	}

	err = rbac.GrantPermission("admin", "article", "read")
	if err != nil {
		t.Errorf("granting duplicate permission should not error, got: %v", err)
	}

	perms, _ := rbac.GetRolePermissions("admin")
	if len(perms) != 1 {
		t.Errorf("expected 1 unique permission, got %d", len(perms))
	}
}

func TestRevokeTwice(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("article", "read")
	rbac.GrantPermission("admin", "article", "read")

	err := rbac.RevokePermission("admin", "article", "read")
	if err != nil {
		t.Fatal(err)
	}

	err = rbac.RevokePermission("admin", "article", "read")
	if !errors.Is(err, ErrPermissionNotFound) {
		t.Errorf("expected ErrPermissionNotFound on second revoke, got %v", err)
	}
}

func TestGetRoleReturnsCopy(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")

	role, _ := rbac.GetRole("admin")
	role.Name = "修改后的名称"

	role2, _ := rbac.GetRole("admin")
	if role2.Name != "管理员" {
		t.Errorf("GetRole should return a copy, but original was modified. Got name: %s", role2.Name)
	}
}

func TestCountMethods(t *testing.T) {
	rbac := NewRBAC()

	if rbac.RoleCount() != 0 {
		t.Error("expected 0 roles")
	}
	if rbac.PermissionCount() != 0 {
		t.Error("expected 0 permissions")
	}
	if rbac.UserCount() != 0 {
		t.Error("expected 0 users")
	}

	rbac.CreateRole("r1", "Role 1", "desc")
	rbac.CreateRole("r2", "Role 2", "desc")
	rbac.RegisterPermission("res", "act")
	rbac.AssignRole("u1", "r1")

	if rbac.RoleCount() != 2 {
		t.Errorf("expected 2 roles, got %d", rbac.RoleCount())
	}
	if rbac.PermissionCount() != 1 {
		t.Errorf("expected 1 permission, got %d", rbac.PermissionCount())
	}
	if rbac.UserCount() != 1 {
		t.Errorf("expected 1 user, got %d", rbac.UserCount())
	}
}

func TestListRolesSorted(t *testing.T) {
	rbac := NewRBAC()

	ids := []string{"zebra", "alpha", "mango", "beta"}
	for _, id := range ids {
		rbac.CreateRole(id, "Name-"+id, "desc")
	}

	roles := rbac.ListRoles()
	if len(roles) != 4 {
		t.Fatalf("expected 4 roles, got %d", len(roles))
	}

	expected := []string{"alpha", "beta", "mango", "zebra"}
	for i, role := range roles {
		if role.ID != expected[i] {
			t.Errorf("expected roles[%d] = %s, got %s", i, expected[i], role.ID)
		}
	}
}

func TestListPermissionsSorted(t *testing.T) {
	rbac := NewRBAC()

	perms := []struct {
		resource string
		action   string
	}{
		{"zebra", "z"},
		{"alpha", "a"},
		{"alpha", "b"},
		{"mango", "a"},
	}
	for _, p := range perms {
		rbac.RegisterPermission(p.resource, p.action)
	}

	result := rbac.ListPermissions()
	if len(result) != 4 {
		t.Fatalf("expected 4 permissions, got %d", len(result))
	}

	if result[0].Resource != "alpha" || result[0].Action != "a" {
		t.Errorf("expected first alpha:a")
	}
	if result[1].Resource != "alpha" || result[1].Action != "b" {
		t.Errorf("expected second alpha:b")
	}
	if result[2].Resource != "mango" || result[2].Action != "a" {
		t.Errorf("expected third mango:a")
	}
	if result[3].Resource != "zebra" || result[3].Action != "z" {
		t.Errorf("expected fourth zebra:z")
	}
}

func TestGetRolePermissionsSorted(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("admin", "管理员", "desc")
	rbac.RegisterPermission("zebra", "z")
	rbac.RegisterPermission("alpha", "a")
	rbac.RegisterPermission("alpha", "b")
	rbac.RegisterPermission("mango", "a")

	rbac.GrantPermission("admin", "zebra", "z")
	rbac.GrantPermission("admin", "alpha", "a")
	rbac.GrantPermission("admin", "alpha", "b")
	rbac.GrantPermission("admin", "mango", "a")

	perms, _ := rbac.GetRolePermissions("admin")
	if len(perms) != 4 {
		t.Fatalf("expected 4 permissions, got %d", len(perms))
	}

	if perms[0].Resource != "alpha" || perms[0].Action != "a" {
		t.Errorf("expected first alpha:a")
	}
	if perms[1].Resource != "alpha" || perms[1].Action != "b" {
		t.Errorf("expected second alpha:b")
	}
	if perms[2].Resource != "mango" || perms[2].Action != "a" {
		t.Errorf("expected third mango:a")
	}
	if perms[3].Resource != "zebra" || perms[3].Action != "z" {
		t.Errorf("expected fourth zebra:z")
	}
}

func TestGetUserPermissionsSorted(t *testing.T) {
	rbac := NewRBAC()

	rbac.CreateRole("r1", "Role 1", "desc")
	rbac.CreateRole("r2", "Role 2", "desc")
	rbac.RegisterPermission("zebra", "z")
	rbac.RegisterPermission("alpha", "a")
	rbac.RegisterPermission("mango", "a")

	rbac.GrantPermission("r1", "zebra", "z")
	rbac.GrantPermission("r2", "alpha", "a")
	rbac.GrantPermission("r2", "mango", "a")

	rbac.AssignRole("user1", "r1")
	rbac.AssignRole("user1", "r2")

	perms, _ := rbac.GetUserPermissions("user1")
	if len(perms) != 3 {
		t.Fatalf("expected 3 permissions, got %d", len(perms))
	}

	if perms[0].Resource != "alpha" || perms[0].Action != "a" {
		t.Errorf("expected first alpha:a, got %s:%s", perms[0].Resource, perms[0].Action)
	}
	if perms[1].Resource != "mango" || perms[1].Action != "a" {
		t.Errorf("expected second mango:a")
	}
	if perms[2].Resource != "zebra" || perms[2].Action != "z" {
		t.Errorf("expected third zebra:z")
	}
}
