package rbac

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrRoleNotFound        = errors.New("role not found")
	ErrRoleExists          = errors.New("role already exists")
	ErrPermissionNotFound  = errors.New("permission not found")
	ErrPermissionExists    = errors.New("permission already exists")
	ErrUserNotFound        = errors.New("user not found")
	ErrEmptyRoleName       = errors.New("role name cannot be empty")
	ErrEmptyRoleID         = errors.New("role id cannot be empty")
	ErrEmptyResource       = errors.New("resource cannot be empty")
	ErrEmptyAction         = errors.New("action cannot be empty")
	ErrEmptyUserID         = errors.New("user id cannot be empty")
	ErrRoleInUse           = errors.New("role is in use and cannot be deleted")
	ErrPermissionInUse     = errors.New("permission is in use and cannot be unregistered")
)

type Role struct {
	ID          string
	Name        string
	Description string
}

type Permission struct {
	Resource string
	Action   string
}

func (p Permission) String() string {
	return fmt.Sprintf("%s:%s", p.Resource, p.Action)
}

func ParsePermission(s string) (Permission, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return Permission{}, fmt.Errorf("invalid permission format: %s", s)
	}
	return Permission{Resource: parts[0], Action: parts[1]}, nil
}

type RoleWithPermissions struct {
	Role
	Permissions []Permission
}

type UserWithRoles struct {
	UserID      string
	Roles       []Role
	Permissions []Permission
}

type Decision struct {
	Allowed bool
	Reason  string
}

type RBAC struct {
	mu                sync.RWMutex
	roles             map[string]*Role
	roleByName        map[string]string
	permissions       map[Permission]bool
	rolePermissions   map[string]map[Permission]bool
	userRoles         map[string]map[string]bool
}

func NewRBAC() *RBAC {
	return &RBAC{
		roles:           make(map[string]*Role),
		roleByName:      make(map[string]string),
		permissions:     make(map[Permission]bool),
		rolePermissions: make(map[string]map[Permission]bool),
		userRoles:       make(map[string]map[string]bool),
	}
}

func (r *RBAC) CreateRole(roleID, name, description string) (*Role, error) {
	if roleID == "" {
		return nil, ErrEmptyRoleID
	}
	if name == "" {
		return nil, ErrEmptyRoleName
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.roles[roleID]; exists {
		return nil, ErrRoleExists
	}

	if _, exists := r.roleByName[name]; exists {
		return nil, ErrRoleExists
	}

	role := &Role{
		ID:          roleID,
		Name:        name,
		Description: description,
	}

	r.roles[roleID] = role
	r.roleByName[name] = roleID
	r.rolePermissions[roleID] = make(map[Permission]bool)

	return role, nil
}

func (r *RBAC) DeleteRole(roleID string) error {
	if roleID == "" {
		return ErrEmptyRoleID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	role, exists := r.roles[roleID]
	if !exists {
		return ErrRoleNotFound
	}

	for _, userRoles := range r.userRoles {
		if userRoles[roleID] {
			return ErrRoleInUse
		}
	}

	delete(r.roles, roleID)
	delete(r.roleByName, role.Name)
	delete(r.rolePermissions, roleID)

	return nil
}

func (r *RBAC) GetRole(roleID string) (*Role, error) {
	if roleID == "" {
		return nil, ErrEmptyRoleID
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	role, exists := r.roles[roleID]
	if !exists {
		return nil, ErrRoleNotFound
	}

	roleCopy := *role
	return &roleCopy, nil
}

func (r *RBAC) GetRoleByName(name string) (*Role, error) {
	if name == "" {
		return nil, ErrEmptyRoleName
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	roleID, exists := r.roleByName[name]
	if !exists {
		return nil, ErrRoleNotFound
	}

	role := r.roles[roleID]
	roleCopy := *role
	return &roleCopy, nil
}

func (r *RBAC) ListRoles() []Role {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roles := make([]Role, 0, len(r.roles))
	for _, role := range r.roles {
		roles = append(roles, *role)
	}

	sort.Slice(roles, func(i, j int) bool {
		return roles[i].ID < roles[j].ID
	})

	return roles
}

func (r *RBAC) RegisterPermission(resource, action string) (Permission, error) {
	if resource == "" {
		return Permission{}, ErrEmptyResource
	}
	if action == "" {
		return Permission{}, ErrEmptyAction
	}

	perm := Permission{Resource: resource, Action: action}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.permissions[perm] {
		return Permission{}, ErrPermissionExists
	}

	r.permissions[perm] = true
	return perm, nil
}

func (r *RBAC) UnregisterPermission(resource, action string) error {
	if resource == "" {
		return ErrEmptyResource
	}
	if action == "" {
		return ErrEmptyAction
	}

	perm := Permission{Resource: resource, Action: action}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.permissions[perm] {
		return ErrPermissionNotFound
	}

	for _, perms := range r.rolePermissions {
		if perms[perm] {
			return ErrPermissionInUse
		}
	}

	delete(r.permissions, perm)
	return nil
}

func (r *RBAC) HasPermission(resource, action string) bool {
	perm := Permission{Resource: resource, Action: action}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.permissions[perm]
}

func (r *RBAC) ListPermissions() []Permission {
	r.mu.RLock()
	defer r.mu.RUnlock()

	perms := make([]Permission, 0, len(r.permissions))
	for perm := range r.permissions {
		perms = append(perms, perm)
	}

	sort.Slice(perms, func(i, j int) bool {
		if perms[i].Resource != perms[j].Resource {
			return perms[i].Resource < perms[j].Resource
		}
		return perms[i].Action < perms[j].Action
	})

	return perms
}

func (r *RBAC) GrantPermission(roleID string, resource, action string) error {
	if roleID == "" {
		return ErrEmptyRoleID
	}
	if resource == "" {
		return ErrEmptyResource
	}
	if action == "" {
		return ErrEmptyAction
	}

	perm := Permission{Resource: resource, Action: action}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.roles[roleID]; !exists {
		return ErrRoleNotFound
	}

	if !r.permissions[perm] {
		return ErrPermissionNotFound
	}

	if _, exists := r.rolePermissions[roleID]; !exists {
		r.rolePermissions[roleID] = make(map[Permission]bool)
	}

	r.rolePermissions[roleID][perm] = true
	return nil
}

func (r *RBAC) RevokePermission(roleID string, resource, action string) error {
	if roleID == "" {
		return ErrEmptyRoleID
	}
	if resource == "" {
		return ErrEmptyResource
	}
	if action == "" {
		return ErrEmptyAction
	}

	perm := Permission{Resource: resource, Action: action}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.roles[roleID]; !exists {
		return ErrRoleNotFound
	}

	if perms, exists := r.rolePermissions[roleID]; exists {
		if !perms[perm] {
			return ErrPermissionNotFound
		}
		delete(perms, perm)
	} else {
		return ErrPermissionNotFound
	}

	return nil
}

func (r *RBAC) GetRolePermissions(roleID string) ([]Permission, error) {
	if roleID == "" {
		return nil, ErrEmptyRoleID
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.roles[roleID]; !exists {
		return nil, ErrRoleNotFound
	}

	permsMap, exists := r.rolePermissions[roleID]
	if !exists {
		return []Permission{}, nil
	}

	perms := make([]Permission, 0, len(permsMap))
	for perm := range permsMap {
		perms = append(perms, perm)
	}

	sort.Slice(perms, func(i, j int) bool {
		if perms[i].Resource != perms[j].Resource {
			return perms[i].Resource < perms[j].Resource
		}
		return perms[i].Action < perms[j].Action
	})

	return perms, nil
}

func (r *RBAC) GetRoleWithPermissions(roleID string) (*RoleWithPermissions, error) {
	role, err := r.GetRole(roleID)
	if err != nil {
		return nil, err
	}

	perms, err := r.GetRolePermissions(roleID)
	if err != nil {
		return nil, err
	}

	return &RoleWithPermissions{
		Role:        *role,
		Permissions: perms,
	}, nil
}

func (r *RBAC) AssignRole(userID string, roleID string) error {
	if userID == "" {
		return ErrEmptyUserID
	}
	if roleID == "" {
		return ErrEmptyRoleID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.roles[roleID]; !exists {
		return ErrRoleNotFound
	}

	if _, exists := r.userRoles[userID]; !exists {
		r.userRoles[userID] = make(map[string]bool)
	}

	r.userRoles[userID][roleID] = true
	return nil
}

func (r *RBAC) RevokeRole(userID string, roleID string) error {
	if userID == "" {
		return ErrEmptyUserID
	}
	if roleID == "" {
		return ErrEmptyRoleID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	roles, exists := r.userRoles[userID]
	if !exists || !roles[roleID] {
		return ErrRoleNotFound
	}

	delete(roles, roleID)
	if len(roles) == 0 {
		delete(r.userRoles, userID)
	}

	return nil
}

func (r *RBAC) GetUserRoles(userID string) ([]Role, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	roleIDs, exists := r.userRoles[userID]
	if !exists {
		return []Role{}, nil
	}

	roles := make([]Role, 0, len(roleIDs))
	for roleID := range roleIDs {
		if role, ok := r.roles[roleID]; ok {
			roles = append(roles, *role)
		}
	}

	sort.Slice(roles, func(i, j int) bool {
		return roles[i].ID < roles[j].ID
	})

	return roles, nil
}

func (r *RBAC) GetUserPermissions(userID string) ([]Permission, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	roleIDs, exists := r.userRoles[userID]
	if !exists {
		return []Permission{}, nil
	}

	permSet := make(map[Permission]bool)
	for roleID := range roleIDs {
		if perms, ok := r.rolePermissions[roleID]; ok {
			for perm := range perms {
				permSet[perm] = true
			}
		}
	}

	perms := make([]Permission, 0, len(permSet))
	for perm := range permSet {
		perms = append(perms, perm)
	}

	sort.Slice(perms, func(i, j int) bool {
		if perms[i].Resource != perms[j].Resource {
			return perms[i].Resource < perms[j].Resource
		}
		return perms[i].Action < perms[j].Action
	})

	return perms, nil
}

func (r *RBAC) GetUserWithRoles(userID string) (*UserWithRoles, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}

	roles, err := r.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}

	perms, err := r.GetUserPermissions(userID)
	if err != nil {
		return nil, err
	}

	return &UserWithRoles{
		UserID:      userID,
		Roles:       roles,
		Permissions: perms,
	}, nil
}

func (r *RBAC) CheckPermission(userID, resource, action string) Decision {
	if userID == "" {
		return Decision{Allowed: false, Reason: ErrEmptyUserID.Error()}
	}
	if resource == "" {
		return Decision{Allowed: false, Reason: ErrEmptyResource.Error()}
	}
	if action == "" {
		return Decision{Allowed: false, Reason: ErrEmptyAction.Error()}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	requested := Permission{Resource: resource, Action: action}

	if !r.permissions[requested] {
		return Decision{
			Allowed: false,
			Reason:  fmt.Sprintf("permission %s is not registered", requested.String()),
		}
	}

	roleIDs, exists := r.userRoles[userID]
	if !exists || len(roleIDs) == 0 {
		return Decision{
			Allowed: false,
			Reason:  fmt.Sprintf("user %s has no roles assigned", userID),
		}
	}

	for roleID := range roleIDs {
		if perms, ok := r.rolePermissions[roleID]; ok {
			if perms[requested] {
				return Decision{Allowed: true, Reason: ""}
			}
		}
	}

	return Decision{
		Allowed: false,
		Reason: fmt.Sprintf(
			"user %s does not have permission %s (required %s:%s)",
			userID, requested.String(), resource, action,
		),
	}
}

func (r *RBAC) RoleCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.roles)
}

func (r *RBAC) PermissionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.permissions)
}

func (r *RBAC) UserCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.userRoles)
}
