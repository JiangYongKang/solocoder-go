package apikey

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.KeyCount() != 0 {
		t.Errorf("expected 0 keys, got %d", m.KeyCount())
	}
}

func TestPermissionString(t *testing.T) {
	p := Permission{Resource: "users", Action: "read"}
	if p.String() != "users:read" {
		t.Errorf("expected 'users:read', got '%s'", p.String())
	}
}

func TestParsePermission(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantPerm  Permission
		wantError bool
	}{
		{"valid", "users:read", Permission{Resource: "users", Action: "read"}, false},
		{"with colons in action", "resource:action:extra", Permission{Resource: "resource", Action: "action:extra"}, false},
		{"empty resource", ":read", Permission{}, true},
		{"empty action", "users:", Permission{}, true},
		{"no colon", "usersread", Permission{}, true},
		{"empty string", "", Permission{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParsePermission(tt.input)
			if tt.wantError {
				if !errors.Is(err, ErrInvalidPermission) {
					t.Errorf("expected ErrInvalidPermission, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if p != tt.wantPerm {
					t.Errorf("expected %v, got %v", tt.wantPerm, p)
				}
			}
		})
	}
}

func TestCreateKeyBasic(t *testing.T) {
	m := NewManager()

	perms := []Permission{
		{Resource: "users", Action: "read"},
		{Resource: "users", Action: "write"},
	}

	opts := CreateKeyOptions{
		Name:        "Test Key",
		Description: "A test API key",
		Permissions: perms,
	}

	created, err := m.CreateKey(opts)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	if created.ID == "" {
		t.Error("expected non-empty key ID")
	}
	if created.Secret == "" {
		t.Error("expected non-empty secret")
	}
	if created.Prefix == "" {
		t.Error("expected non-empty prefix")
	}

	if len(created.ID) != KeyIDLength*2 {
		t.Errorf("expected key ID length %d, got %d", KeyIDLength*2, len(created.ID))
	}

	expectedPrefixLen := len(SecretPrefix) + PrefixLength
	if len(created.Prefix) != expectedPrefixLen {
		t.Errorf("expected prefix length %d, got %d", expectedPrefixLen, len(created.Prefix))
	}

	if len(created.Secret) <= len(SecretPrefix) {
		t.Error("secret should be longer than prefix")
	}

	if created.Secret[:len(SecretPrefix)] != SecretPrefix {
		t.Errorf("expected secret to start with '%s'", SecretPrefix)
	}

	if created.Secret[:expectedPrefixLen] != created.Prefix {
		t.Error("prefix should be the beginning of secret")
	}

	if m.KeyCount() != 1 {
		t.Errorf("expected 1 key, got %d", m.KeyCount())
	}
}

func TestCreateKeyOnlyOnceReturnsSecret(t *testing.T) {
	m := NewManager()

	created, err := m.CreateKey(CreateKeyOptions{})
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	meta, err := m.GetKeyMeta(created.ID)
	if err != nil {
		t.Fatalf("GetKeyMeta failed: %v", err)
	}

	_ = meta
}

func TestCreateKeyWithMaxUses(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Name:    "Limited Key",
		MaxUses: 100,
	}

	created, err := m.CreateKey(opts)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	meta, err := m.GetKeyMeta(created.ID)
	if err != nil {
		t.Fatalf("GetKeyMeta failed: %v", err)
	}

	if meta.MaxUses != 100 {
		t.Errorf("expected MaxUses 100, got %d", meta.MaxUses)
	}
	if meta.RemainingUses != 100 {
		t.Errorf("expected RemainingUses 100, got %d", meta.RemainingUses)
	}
}

func TestCreateKeyWithUnlimitedUses(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Name:    "Unlimited Key",
		MaxUses: 0,
	}

	created, err := m.CreateKey(opts)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	meta, err := m.GetKeyMeta(created.ID)
	if err != nil {
		t.Fatalf("GetKeyMeta failed: %v", err)
	}

	if meta.MaxUses != 0 {
		t.Errorf("expected MaxUses 0, got %d", meta.MaxUses)
	}
	if meta.RemainingUses != -1 {
		t.Errorf("expected RemainingUses -1, got %d", meta.RemainingUses)
	}
}

func TestCreateKeyWithNegativeMaxUses(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Name:    "Bad Key",
		MaxUses: -1,
	}

	_, err := m.CreateKey(opts)
	if !errors.Is(err, ErrMaxUsesZeroOrNegative) {
		t.Errorf("expected ErrMaxUsesZeroOrNegative, got %v", err)
	}
}

func TestCreateKeyWithTTL(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Name: "TTL Key",
		TTL:  24 * time.Hour,
	}

	created, err := m.CreateKey(opts)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	meta, err := m.GetKeyMeta(created.ID)
	if err != nil {
		t.Fatalf("GetKeyMeta failed: %v", err)
	}

	if !meta.HasExpiration {
		t.Error("expected HasExpiration to be true")
	}
	if meta.ExpiresAt.Before(time.Now().Add(23*time.Hour)) || meta.ExpiresAt.After(time.Now().Add(25*time.Hour)) {
		t.Errorf("unexpected ExpiresAt: %v", meta.ExpiresAt)
	}
	if meta.Status != StatusActive {
		t.Errorf("expected StatusActive, got %s", meta.Status)
	}
}

func TestCreateKeyWithNegativeTTL(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Name: "Bad TTL Key",
		TTL:  -1 * time.Second,
	}

	_, err := m.CreateKey(opts)
	if !errors.Is(err, ErrNegativeTTL) {
		t.Errorf("expected ErrNegativeTTL, got %v", err)
	}
}

func TestCreateKeyWithExpiresAt(t *testing.T) {
	m := NewManager()

	futureTime := time.Now().Add(48 * time.Hour)

	opts := CreateKeyOptions{
		Name:          "ExpiresAt Key",
		ExpiresAt:     futureTime,
		HasExpiration: true,
	}

	created, err := m.CreateKey(opts)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	meta, err := m.GetKeyMeta(created.ID)
	if err != nil {
		t.Fatalf("GetKeyMeta failed: %v", err)
	}

	if !meta.HasExpiration {
		t.Error("expected HasExpiration to be true")
	}
}

func TestCreateKeyWithExpiresAtInPast(t *testing.T) {
	m := NewManager()

	pastTime := time.Now().Add(-1 * time.Hour)

	opts := CreateKeyOptions{
		Name:          "Past Key",
		ExpiresAt:     pastTime,
		HasExpiration: true,
	}

	_, err := m.CreateKey(opts)
	if !errors.Is(err, ErrExpiresAtInThePast) {
		t.Errorf("expected ErrExpiresAtInThePast, got %v", err)
	}
}

func TestCreateKeyWithoutExpiration(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Name: "Never Expires Key",
	}

	created, err := m.CreateKey(opts)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	meta, err := m.GetKeyMeta(created.ID)
	if err != nil {
		t.Fatalf("GetKeyMeta failed: %v", err)
	}

	if meta.HasExpiration {
		t.Error("expected HasExpiration to be false")
	}
	if meta.Status != StatusActive {
		t.Errorf("expected StatusActive, got %s", meta.Status)
	}
}

func TestCreateKeyWithEmptyResource(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Permissions: []Permission{
			{Resource: "", Action: "read"},
		},
	}

	_, err := m.CreateKey(opts)
	if !errors.Is(err, ErrEmptyResource) {
		t.Errorf("expected ErrEmptyResource, got %v", err)
	}
}

func TestCreateKeyWithEmptyAction(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Permissions: []Permission{
			{Resource: "users", Action: ""},
		},
	}

	_, err := m.CreateKey(opts)
	if !errors.Is(err, ErrEmptyAction) {
		t.Errorf("expected ErrEmptyAction, got %v", err)
	}
}

func TestGetKeyMeta(t *testing.T) {
	m := NewManager()

	created, err := m.CreateKey(CreateKeyOptions{Name: "My Key"})
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	meta, err := m.GetKeyMeta(created.ID)
	if err != nil {
		t.Fatalf("GetKeyMeta failed: %v", err)
	}

	if meta.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, meta.ID)
	}
	if meta.Prefix != created.Prefix {
		t.Errorf("expected Prefix %s, got %s", created.Prefix, meta.Prefix)
	}
	if meta.Name != "My Key" {
		t.Errorf("expected Name 'My Key', got '%s'", meta.Name)
	}
	if meta.Status != StatusActive {
		t.Errorf("expected StatusActive, got %s", meta.Status)
	}
}

func TestGetKeyMetaNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetKeyMeta("nonexistent-id")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestGetKeyMetaEmptyID(t *testing.T) {
	m := NewManager()

	_, err := m.GetKeyMeta("")
	if !errors.Is(err, ErrEmptyKeyID) {
		t.Errorf("expected ErrEmptyKeyID, got %v", err)
	}
}

func TestListKeysByPrefix(t *testing.T) {
	m := NewManager()

	created1, _ := m.CreateKey(CreateKeyOptions{Name: "Key 1"})
	created2, _ := m.CreateKey(CreateKeyOptions{Name: "Key 2"})
	created3, _ := m.CreateKey(CreateKeyOptions{Name: "Key 3"})

	keys1, err := m.ListKeysByPrefix(created1.Prefix)
	if err != nil {
		t.Fatalf("ListKeysByPrefix failed: %v", err)
	}
	if len(keys1) != 1 {
		t.Errorf("expected 1 key for prefix %s, got %d", created1.Prefix, len(keys1))
	}

	_ = created2
	_ = created3
}

func TestListKeysByPrefixEmptyPrefix(t *testing.T) {
	m := NewManager()

	_, err := m.ListKeysByPrefix("")
	if !errors.Is(err, ErrEmptyPrefix) {
		t.Errorf("expected ErrEmptyPrefix, got %v", err)
	}
}

func TestListKeysByPrefixNotFound(t *testing.T) {
	m := NewManager()

	keys, err := m.ListKeysByPrefix("sk_00000000")
	if err != nil {
		t.Fatalf("ListKeysByPrefix failed: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestListAllKeys(t *testing.T) {
	m := NewManager()

	for i := 0; i < 5; i++ {
		_, err := m.CreateKey(CreateKeyOptions{Name: fmt.Sprintf("Key %d", i)})
		if err != nil {
			t.Fatalf("CreateKey %d failed: %v", i, err)
		}
	}

	keys := m.ListAllKeys()
	if len(keys) != 5 {
		t.Errorf("expected 5 keys, got %d", len(keys))
	}
}

func TestListAllKeysEmpty(t *testing.T) {
	m := NewManager()

	keys := m.ListAllKeys()
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestRevokeKey(t *testing.T) {
	m := NewManager()

	created, err := m.CreateKey(CreateKeyOptions{Name: "To Revoke"})
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	err = m.RevokeKey(created.ID, "compromised in data breach")
	if err != nil {
		t.Fatalf("RevokeKey failed: %v", err)
	}

	meta, err := m.GetKeyMeta(created.ID)
	if err != nil {
		t.Fatalf("GetKeyMeta failed: %v", err)
	}

	if !meta.Revoked {
		t.Error("expected Revoked to be true")
	}
	if meta.RevokeReason != "compromised in data breach" {
		t.Errorf("expected revoke reason, got '%s'", meta.RevokeReason)
	}
	if meta.RevokedAt.IsZero() {
		t.Error("expected RevokedAt to be set")
	}
	if meta.Status != StatusRevoked {
		t.Errorf("expected StatusRevoked, got %s", meta.Status)
	}
}

func TestRevokeKeyNotFound(t *testing.T) {
	m := NewManager()

	err := m.RevokeKey("nonexistent", "reason")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestRevokeKeyEmptyID(t *testing.T) {
	m := NewManager()

	err := m.RevokeKey("", "reason")
	if !errors.Is(err, ErrEmptyKeyID) {
		t.Errorf("expected ErrEmptyKeyID, got %v", err)
	}
}

func TestRevokeKeyEmptyReason(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})

	err := m.RevokeKey(created.ID, "")
	if !errors.Is(err, ErrEmptyRevokeReason) {
		t.Errorf("expected ErrEmptyRevokeReason, got %v", err)
	}
}

func TestRevokeKeyAlreadyRevoked(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})
	m.RevokeKey(created.ID, "first reason")

	err := m.RevokeKey(created.ID, "second reason")
	if !errors.Is(err, ErrAlreadyRevoked) {
		t.Errorf("expected ErrAlreadyRevoked, got %v", err)
	}
}

func TestVerifyKeyValid(t *testing.T) {
	m := NewManager()

	created, err := m.CreateKey(CreateKeyOptions{})
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	result := m.VerifyKey(created.Secret)
	if !result.Valid {
		t.Errorf("expected valid key, got invalid: %v", result.Reason)
	}
	if result.KeyMeta == nil {
		t.Fatal("expected non-nil KeyMeta")
	}
	if result.KeyMeta.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, result.KeyMeta.ID)
	}
	if result.KeyMeta.UsedCount != 1 {
		t.Errorf("expected UsedCount 1 after verification, got %d", result.KeyMeta.UsedCount)
	}
}

func TestVerifyKeyEmptySecret(t *testing.T) {
	m := NewManager()

	result := m.VerifyKey("")
	if result.Valid {
		t.Error("expected invalid key for empty secret")
	}
	if !errors.Is(result.Reason, ErrEmptySecret) {
		t.Errorf("expected ErrEmptySecret, got %v", result.Reason)
	}
}

func TestVerifyKeyInvalidSecret(t *testing.T) {
	m := NewManager()

	m.CreateKey(CreateKeyOptions{})

	result := m.VerifyKey("sk_invalidsecret123456789012345678901234567890")
	if result.Valid {
		t.Error("expected invalid key")
	}
	if !errors.Is(result.Reason, ErrInvalidSecret) {
		t.Errorf("expected ErrInvalidSecret, got %v", result.Reason)
	}
}

func TestVerifyKeyRevoked(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})
	m.RevokeKey(created.ID, "test revoke")

	result := m.VerifyKey(created.Secret)
	if result.Valid {
		t.Error("expected revoked key to be invalid")
	}
	if !errors.Is(result.Reason, ErrKeyRevoked) {
		t.Errorf("expected ErrKeyRevoked, got %v", result.Reason)
	}
	if result.KeyMeta == nil {
		t.Error("expected KeyMeta to be present for revoked key")
	}
}

func TestVerifyKeyExpired(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		TTL: 10 * time.Millisecond,
	}

	created, _ := m.CreateKey(opts)

	time.Sleep(50 * time.Millisecond)

	result := m.VerifyKey(created.Secret)
	if result.Valid {
		t.Error("expected expired key to be invalid")
	}
	if !errors.Is(result.Reason, ErrKeyExpired) {
		t.Errorf("expected ErrKeyExpired, got %v", result.Reason)
	}
}

func TestVerifyKeyUsageExceeded(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		MaxUses: 2,
	}

	created, _ := m.CreateKey(opts)

	r1 := m.VerifyKey(created.Secret)
	if !r1.Valid {
		t.Fatalf("first verify should be valid: %v", r1.Reason)
	}

	r2 := m.VerifyKey(created.Secret)
	if !r2.Valid {
		t.Fatalf("second verify should be valid: %v", r2.Reason)
	}

	r3 := m.VerifyKey(created.Secret)
	if r3.Valid {
		t.Error("third verify should be invalid (usage exceeded)")
	}
	if !errors.Is(r3.Reason, ErrUsageLimitExceeded) {
		t.Errorf("expected ErrUsageLimitExceeded, got %v", r3.Reason)
	}
}

func TestCheckAccessAllowed(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Permissions: []Permission{
			{Resource: "users", Action: "read"},
			{Resource: "users", Action: "write"},
		},
	}

	created, _ := m.CreateKey(opts)

	result := m.CheckAccess(created.ID, Permission{Resource: "users", Action: "read"})
	if !result.Allowed {
		t.Errorf("expected access allowed, got denied: %v", result.Reason)
	}
}

func TestCheckAccessDeniedNoPermission(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Permissions: []Permission{
			{Resource: "users", Action: "read"},
		},
	}

	created, _ := m.CreateKey(opts)

	result := m.CheckAccess(created.ID, Permission{Resource: "users", Action: "delete"})
	if result.Allowed {
		t.Error("expected access denied")
	}
}

func TestCheckAccessRevokedKey(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Permissions: []Permission{
			{Resource: "users", Action: "read"},
		},
	}

	created, _ := m.CreateKey(opts)
	m.RevokeKey(created.ID, "test")

	result := m.CheckAccess(created.ID, Permission{Resource: "users", Action: "read"})
	if result.Allowed {
		t.Error("expected access denied for revoked key")
	}
	if !errors.Is(result.Reason, ErrKeyRevoked) {
		t.Errorf("expected ErrKeyRevoked, got %v", result.Reason)
	}
}

func TestCheckAccessExpiredKey(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		TTL: 10 * time.Millisecond,
		Permissions: []Permission{
			{Resource: "users", Action: "read"},
		},
	}

	created, _ := m.CreateKey(opts)
	time.Sleep(50 * time.Millisecond)

	result := m.CheckAccess(created.ID, Permission{Resource: "users", Action: "read"})
	if result.Allowed {
		t.Error("expected access denied for expired key")
	}
	if !errors.Is(result.Reason, ErrKeyExpired) {
		t.Errorf("expected ErrKeyExpired, got %v", result.Reason)
	}
}

func TestCheckAccessDepletedKey(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		MaxUses: 1,
		Permissions: []Permission{
			{Resource: "users", Action: "read"},
		},
	}

	created, _ := m.CreateKey(opts)
	m.VerifyKey(created.Secret)

	result := m.CheckAccess(created.ID, Permission{Resource: "users", Action: "read"})
	if result.Allowed {
		t.Error("expected access denied for depleted key")
	}
	if !errors.Is(result.Reason, ErrUsageLimitExceeded) {
		t.Errorf("expected ErrUsageLimitExceeded, got %v", result.Reason)
	}
}

func TestCheckAccessEmptyID(t *testing.T) {
	m := NewManager()

	result := m.CheckAccess("", Permission{Resource: "users", Action: "read"})
	if result.Allowed {
		t.Error("expected access denied for empty ID")
	}
	if !errors.Is(result.Reason, ErrEmptyKeyID) {
		t.Errorf("expected ErrEmptyKeyID, got %v", result.Reason)
	}
}

func TestCheckAccessEmptyResource(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})

	result := m.CheckAccess(created.ID, Permission{Resource: "", Action: "read"})
	if result.Allowed {
		t.Error("expected access denied for empty resource")
	}
	if !errors.Is(result.Reason, ErrEmptyResource) {
		t.Errorf("expected ErrEmptyResource, got %v", result.Reason)
	}
}

func TestCheckAccessEmptyAction(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})

	result := m.CheckAccess(created.ID, Permission{Resource: "users", Action: ""})
	if result.Allowed {
		t.Error("expected access denied for empty action")
	}
	if !errors.Is(result.Reason, ErrEmptyAction) {
		t.Errorf("expected ErrEmptyAction, got %v", result.Reason)
	}
}

func TestCheckAccessKeyNotFound(t *testing.T) {
	m := NewManager()

	result := m.CheckAccess("nonexistent", Permission{Resource: "users", Action: "read"})
	if result.Allowed {
		t.Error("expected access denied for nonexistent key")
	}
	if !errors.Is(result.Reason, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", result.Reason)
	}
}

func TestVerifyAndCheckAccess(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		Permissions: []Permission{
			{Resource: "users", Action: "read"},
		},
	}

	created, _ := m.CreateKey(opts)

	meta, access := m.VerifyAndCheckAccess(created.Secret, Permission{Resource: "users", Action: "read"})
	if meta == nil {
		t.Fatal("expected non-nil meta")
	}
	if !access.Allowed {
		t.Errorf("expected access allowed, got denied: %v", access.Reason)
	}
}

func TestVerifyAndCheckAccessBadSecret(t *testing.T) {
	m := NewManager()

	_, access := m.VerifyAndCheckAccess("bad_secret", Permission{Resource: "users", Action: "read"})
	if access.Allowed {
		t.Error("expected access denied")
	}
}

func TestIncrementUsage(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		MaxUses: 10,
	}

	created, _ := m.CreateKey(opts)

	for i := int64(1); i <= 5; i++ {
		count, err := m.IncrementUsage(created.ID)
		if err != nil {
			t.Fatalf("IncrementUsage %d failed: %v", i, err)
		}
		if count != i {
			t.Errorf("expected count %d, got %d", i, count)
		}
	}

	remaining, err := m.GetRemainingUses(created.ID)
	if err != nil {
		t.Fatalf("GetRemainingUses failed: %v", err)
	}
	if remaining != 5 {
		t.Errorf("expected 5 remaining uses, got %d", remaining)
	}
}

func TestIncrementUsageUnlimited(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{MaxUses: 0})

	for i := int64(1); i <= 100; i++ {
		count, err := m.IncrementUsage(created.ID)
		if err != nil {
			t.Fatalf("IncrementUsage %d failed: %v", i, err)
		}
		if count != i {
			t.Errorf("expected count %d, got %d", i, count)
		}
	}

	remaining, _ := m.GetRemainingUses(created.ID)
	if remaining != -1 {
		t.Errorf("expected -1 remaining uses, got %d", remaining)
	}
}

func TestIncrementUsageEmptyID(t *testing.T) {
	m := NewManager()

	_, err := m.IncrementUsage("")
	if !errors.Is(err, ErrEmptyKeyID) {
		t.Errorf("expected ErrEmptyKeyID, got %v", err)
	}
}

func TestIncrementUsageNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.IncrementUsage("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestIncrementUsageRevoked(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})
	m.RevokeKey(created.ID, "reason")

	_, err := m.IncrementUsage(created.ID)
	if !errors.Is(err, ErrKeyRevoked) {
		t.Errorf("expected ErrKeyRevoked, got %v", err)
	}
}

func TestIncrementUsageExpired(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{TTL: 10 * time.Millisecond})
	time.Sleep(50 * time.Millisecond)

	_, err := m.IncrementUsage(created.ID)
	if !errors.Is(err, ErrKeyExpired) {
		t.Errorf("expected ErrKeyExpired, got %v", err)
	}
}

func TestIncrementUsageExceeded(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{MaxUses: 1})

	_, err := m.IncrementUsage(created.ID)
	if err != nil {
		t.Fatalf("first increment failed: %v", err)
	}

	_, err = m.IncrementUsage(created.ID)
	if !errors.Is(err, ErrUsageLimitExceeded) {
		t.Errorf("expected ErrUsageLimitExceeded, got %v", err)
	}
}

func TestGetRemainingUses(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{MaxUses: 50})

	m.IncrementUsage(created.ID)
	m.IncrementUsage(created.ID)

	remaining, err := m.GetRemainingUses(created.ID)
	if err != nil {
		t.Fatalf("GetRemainingUses failed: %v", err)
	}
	if remaining != 48 {
		t.Errorf("expected 48 remaining, got %d", remaining)
	}
}

func TestGetRemainingUsesEmptyID(t *testing.T) {
	m := NewManager()

	_, err := m.GetRemainingUses("")
	if !errors.Is(err, ErrEmptyKeyID) {
		t.Errorf("expected ErrEmptyKeyID, got %v", err)
	}
}

func TestGetRemainingUsesNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetRemainingUses("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestGetRemainingTime(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{TTL: 1 * time.Hour})

	remaining, has, err := m.GetRemainingTime(created.ID)
	if err != nil {
		t.Fatalf("GetRemainingTime failed: %v", err)
	}
	if !has {
		t.Error("expected has expiration")
	}
	if remaining <= 0 {
		t.Errorf("expected positive remaining time, got %v", remaining)
	}
}

func TestGetRemainingTimeNoExpiration(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})

	_, has, err := m.GetRemainingTime(created.ID)
	if err != nil {
		t.Fatalf("GetRemainingTime failed: %v", err)
	}
	if has {
		t.Error("expected no expiration")
	}
}

func TestGetRemainingTimeEmptyID(t *testing.T) {
	m := NewManager()

	_, _, err := m.GetRemainingTime("")
	if !errors.Is(err, ErrEmptyKeyID) {
		t.Errorf("expected ErrEmptyKeyID, got %v", err)
	}
}

func TestGetRemainingTimeNotFound(t *testing.T) {
	m := NewManager()

	_, _, err := m.GetRemainingTime("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestSetExpiresAt(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})

	newExpiry := time.Now().Add(72 * time.Hour)
	err := m.SetExpiresAt(created.ID, newExpiry)
	if err != nil {
		t.Fatalf("SetExpiresAt failed: %v", err)
	}

	meta, _ := m.GetKeyMeta(created.ID)
	if !meta.HasExpiration {
		t.Error("expected HasExpiration to be true")
	}
}

func TestSetExpiresAtInPast(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})

	past := time.Now().Add(-1 * time.Hour)
	err := m.SetExpiresAt(created.ID, past)
	if !errors.Is(err, ErrExpiresAtInThePast) {
		t.Errorf("expected ErrExpiresAtInThePast, got %v", err)
	}
}

func TestSetExpiresAtEmptyID(t *testing.T) {
	m := NewManager()

	err := m.SetExpiresAt("", time.Now().Add(time.Hour))
	if !errors.Is(err, ErrEmptyKeyID) {
		t.Errorf("expected ErrEmptyKeyID, got %v", err)
	}
}

func TestSetExpiresAtNotFound(t *testing.T) {
	m := NewManager()

	err := m.SetExpiresAt("nonexistent", time.Now().Add(time.Hour))
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestSetExpiresAtRevoked(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})
	m.RevokeKey(created.ID, "reason")

	err := m.SetExpiresAt(created.ID, time.Now().Add(time.Hour))
	if !errors.Is(err, ErrKeyRevoked) {
		t.Errorf("expected ErrKeyRevoked, got %v", err)
	}
}

func TestSetTTL(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})

	err := m.SetTTL(created.ID, 48*time.Hour)
	if err != nil {
		t.Fatalf("SetTTL failed: %v", err)
	}

	meta, _ := m.GetKeyMeta(created.ID)
	if !meta.HasExpiration {
		t.Error("expected HasExpiration to be true")
	}
}

func TestSetTTLRemoveExpiration(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{TTL: time.Hour})

	err := m.SetTTL(created.ID, 0)
	if err != nil {
		t.Fatalf("SetTTL failed: %v", err)
	}

	meta, _ := m.GetKeyMeta(created.ID)
	if meta.HasExpiration {
		t.Error("expected HasExpiration to be false after setting TTL to 0")
	}
	if meta.Status != StatusActive {
		t.Errorf("expected StatusActive, got %s", meta.Status)
	}
}

func TestSetTTLNegative(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})

	err := m.SetTTL(created.ID, -1*time.Second)
	if !errors.Is(err, ErrNegativeTTL) {
		t.Errorf("expected ErrNegativeTTL, got %v", err)
	}
}

func TestSetTTLEmptyID(t *testing.T) {
	m := NewManager()

	err := m.SetTTL("", time.Hour)
	if !errors.Is(err, ErrEmptyKeyID) {
		t.Errorf("expected ErrEmptyKeyID, got %v", err)
	}
}

func TestSetTTLNotFound(t *testing.T) {
	m := NewManager()

	err := m.SetTTL("nonexistent", time.Hour)
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestSetTTLRevoked(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{})
	m.RevokeKey(created.ID, "reason")

	err := m.SetTTL(created.ID, time.Hour)
	if !errors.Is(err, ErrKeyRevoked) {
		t.Errorf("expected ErrKeyRevoked, got %v", err)
	}
}

func TestAPIKeyStatusTransitions(t *testing.T) {
	m := NewManager()

	t.Run("active state", func(t *testing.T) {
		created, _ := m.CreateKey(CreateKeyOptions{})
		meta, _ := m.GetKeyMeta(created.ID)
		if meta.Status != StatusActive {
			t.Errorf("expected StatusActive, got %s", meta.Status)
		}
	})

	t.Run("depleted state", func(t *testing.T) {
		created, _ := m.CreateKey(CreateKeyOptions{MaxUses: 1})
		m.VerifyKey(created.Secret)
		meta, _ := m.GetKeyMeta(created.ID)
		if meta.Status != StatusDepleted {
			t.Errorf("expected StatusDepleted, got %s", meta.Status)
		}
	})

	t.Run("expired state", func(t *testing.T) {
		created, _ := m.CreateKey(CreateKeyOptions{TTL: 10 * time.Millisecond})
		time.Sleep(50 * time.Millisecond)
		meta, _ := m.GetKeyMeta(created.ID)
		if meta.Status != StatusExpired {
			t.Errorf("expected StatusExpired, got %s", meta.Status)
		}
	})

	t.Run("revoked state", func(t *testing.T) {
		created, _ := m.CreateKey(CreateKeyOptions{})
		m.RevokeKey(created.ID, "reason")
		meta, _ := m.GetKeyMeta(created.ID)
		if meta.Status != StatusRevoked {
			t.Errorf("expected StatusRevoked, got %s", meta.Status)
		}
	})

	t.Run("revoked takes priority", func(t *testing.T) {
		created, _ := m.CreateKey(CreateKeyOptions{TTL: 10 * time.Millisecond, MaxUses: 1})
		m.VerifyKey(created.Secret)
		time.Sleep(50 * time.Millisecond)
		m.RevokeKey(created.ID, "reason")
		meta, _ := m.GetKeyMeta(created.ID)
		if meta.Status != StatusRevoked {
			t.Errorf("revoked should take priority, expected StatusRevoked, got %s", meta.Status)
		}
	})
}

func TestConcurrentVerifyKey(t *testing.T) {
	m := NewManager()

	opts := CreateKeyOptions{
		MaxUses: 1000,
	}

	created, _ := m.CreateKey(opts)

	var wg sync.WaitGroup
	var successful int32
	var failed int32

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := m.VerifyKey(created.Secret)
			if result.Valid {
				atomic.AddInt32(&successful, 1)
			} else {
				atomic.AddInt32(&failed, 1)
			}
		}()
	}

	wg.Wait()

	if successful != 1000 {
		t.Errorf("expected 1000 successful verifies, got %d (failed: %d)", successful, failed)
	}

	meta, _ := m.GetKeyMeta(created.ID)
	if meta.UsedCount != 1000 {
		t.Errorf("expected UsedCount 1000, got %d", meta.UsedCount)
	}
}

func TestConcurrentIncrementUsage(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{MaxUses: 500})

	var wg sync.WaitGroup
	var errorCount int32

	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.IncrementUsage(created.ID)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			}
		}()
	}

	wg.Wait()

	if errorCount != 0 {
		t.Errorf("expected 0 errors, got %d", errorCount)
	}

	remaining, _ := m.GetRemainingUses(created.ID)
	if remaining != 0 {
		t.Errorf("expected 0 remaining uses, got %d", remaining)
	}
}

func TestConcurrentIncrementUsageExact(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{MaxUses: 100})

	var wg sync.WaitGroup
	var exceeded int32

	for i := 0; i < 150; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.IncrementUsage(created.ID)
			if errors.Is(err, ErrUsageLimitExceeded) {
				atomic.AddInt32(&exceeded, 1)
			}
		}()
	}

	wg.Wait()

	meta, _ := m.GetKeyMeta(created.ID)
	if meta.UsedCount != 100 {
		t.Errorf("expected UsedCount exactly 100, got %d", meta.UsedCount)
	}
	if exceeded != 50 {
		t.Errorf("expected 50 exceeded errors, got %d", exceeded)
	}
}

func TestConcurrentCreateKeys(t *testing.T) {
	m := NewManager()

	var wg sync.WaitGroup
	numKeys := 50

	for i := 0; i < numKeys; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := m.CreateKey(CreateKeyOptions{
				Name: fmt.Sprintf("concurrent-key-%d", i),
			})
			if err != nil {
				t.Errorf("CreateKey %d failed: %v", i, err)
			}
		}(i)
	}

	wg.Wait()

	if m.KeyCount() != numKeys {
		t.Errorf("expected %d keys, got %d", numKeys, m.KeyCount())
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	m := NewManager()

	created, _ := m.CreateKey(CreateKeyOptions{
		Permissions: []Permission{
			{Resource: "data", Action: "read"},
			{Resource: "data", Action: "write"},
		},
	})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					m.VerifyKey(created.Secret)
				}
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					m.GetKeyMeta(created.ID)
				}
			}
		}()
	}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					m.CheckAccess(created.ID, Permission{Resource: "data", Action: "read"})
				}
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)

	close(stop)
	wg.Wait()
}

func TestFullWorkflow(t *testing.T) {
	m := NewManager()

	perms := []Permission{
		{Resource: "articles", Action: "read"},
		{Resource: "articles", Action: "write"},
		{Resource: "users", Action: "read"},
	}

	opts := CreateKeyOptions{
		Name:           "Production API Key",
		Description:    "Main key for production service",
		Permissions:    perms,
		MaxUses:        1000,
		TTL:            30 * 24 * time.Hour,
	}

	created, err := m.CreateKey(opts)
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}

	if created.Secret == "" {
		t.Fatal("secret should not be empty")
	}

	verify1 := m.VerifyKey(created.Secret)
	if !verify1.Valid {
		t.Fatalf("first verify failed: %v", verify1.Reason)
	}

	access1 := m.CheckAccess(created.ID, Permission{Resource: "articles", Action: "read"})
	if !access1.Allowed {
		t.Errorf("articles:read should be allowed: %v", access1.Reason)
	}

	access2 := m.CheckAccess(created.ID, Permission{Resource: "articles", Action: "delete"})
	if access2.Allowed {
		t.Error("articles:delete should be denied")
	}

	for i := 0; i < 99; i++ {
		m.VerifyKey(created.Secret)
	}

	remaining, _ := m.GetRemainingUses(created.ID)
	if remaining != 900 {
		t.Errorf("expected 900 remaining uses, got %d", remaining)
	}

	newExpiry := time.Now().Add(60 * 24 * time.Hour)
	err = m.SetExpiresAt(created.ID, newExpiry)
	if err != nil {
		t.Fatalf("SetExpiresAt failed: %v", err)
	}

	timeLeft, hasExp, _ := m.GetRemainingTime(created.ID)
	if !hasExp {
		t.Error("expected to have expiration")
	}
	if timeLeft <= 0 {
		t.Errorf("expected positive time left, got %v", timeLeft)
	}

	err = m.RevokeKey(created.ID, "Emergency: key compromised")
	if err != nil {
		t.Fatalf("RevokeKey failed: %v", err)
	}

	verifyAfter := m.VerifyKey(created.Secret)
	if verifyAfter.Valid {
		t.Error("verify after revoke should be invalid")
	}
	if !errors.Is(verifyAfter.Reason, ErrKeyRevoked) {
		t.Errorf("expected ErrKeyRevoked, got %v", verifyAfter.Reason)
	}

	meta, _ := m.GetKeyMeta(created.ID)
	if meta.Status != StatusRevoked {
		t.Errorf("expected StatusRevoked, got %s", meta.Status)
	}
	if meta.RevokeReason != "Emergency: key compromised" {
		t.Errorf("unexpected revoke reason: %s", meta.RevokeReason)
	}
}

func TestEdgeCases(t *testing.T) {
	m := NewManager()

	t.Run("name and description with special chars", func(t *testing.T) {
		created, err := m.CreateKey(CreateKeyOptions{
			Name:        "Key-With_Special@Chars#2024!",
			Description: "Description with \nnewlines and\ttabs",
		})
		if err != nil {
			t.Fatalf("CreateKey failed: %v", err)
		}
		meta, _ := m.GetKeyMeta(created.ID)
		if meta.Name != "Key-With_Special@Chars#2024!" {
			t.Errorf("name corrupted")
		}
	})

	t.Run("large number of permissions", func(t *testing.T) {
		perms := make([]Permission, 100)
		for i := 0; i < 100; i++ {
			perms[i] = Permission{
				Resource: fmt.Sprintf("resource_%d", i),
				Action:   fmt.Sprintf("action_%d", i),
			}
		}
		created, err := m.CreateKey(CreateKeyOptions{Permissions: perms})
		if err != nil {
			t.Fatalf("CreateKey failed: %v", err)
		}
		meta, _ := m.GetKeyMeta(created.ID)
		if len(meta.Permissions) != 100 {
			t.Errorf("expected 100 permissions, got %d", len(meta.Permissions))
		}
	})

	t.Run("verify key with full scan", func(t *testing.T) {
		m2 := NewManager()
		for i := 0; i < 10; i++ {
			m2.CreateKey(CreateKeyOptions{})
		}
		target, _ := m2.CreateKey(CreateKeyOptions{})
		result := m2.VerifyKey(target.Secret)
		if !result.Valid {
			t.Errorf("expected valid key via full scan: %v", result.Reason)
		}
	})

	t.Run("double revoke protection", func(t *testing.T) {
		created, _ := m.CreateKey(CreateKeyOptions{})
		err := m.RevokeKey(created.ID, "reason 1")
		if err != nil {
			t.Fatalf("first revoke failed: %v", err)
		}
		err = m.RevokeKey(created.ID, "reason 2")
		if !errors.Is(err, ErrAlreadyRevoked) {
			t.Errorf("expected ErrAlreadyRevoked, got %v", err)
		}
		meta, _ := m.GetKeyMeta(created.ID)
		if meta.RevokeReason != "reason 1" {
			t.Errorf("revoke reason should not change, got '%s'", meta.RevokeReason)
		}
	})

	t.Run("permissions sorted in meta", func(t *testing.T) {
		perms := []Permission{
			{Resource: "zebra", Action: "z"},
			{Resource: "alpha", Action: "b"},
			{Resource: "alpha", Action: "a"},
			{Resource: "mango", Action: "a"},
		}
		created, _ := m.CreateKey(CreateKeyOptions{Permissions: perms})
		meta, _ := m.GetKeyMeta(created.ID)
		if meta.Permissions[0].Resource != "alpha" || meta.Permissions[0].Action != "a" {
			t.Errorf("expected first perm alpha:a, got %s:%s", meta.Permissions[0].Resource, meta.Permissions[0].Action)
		}
		if meta.Permissions[1].Resource != "alpha" || meta.Permissions[1].Action != "b" {
			t.Errorf("expected second perm alpha:b")
		}
		if meta.Permissions[2].Resource != "mango" {
			t.Errorf("expected third perm mango")
		}
		if meta.Permissions[3].Resource != "zebra" {
			t.Errorf("expected fourth perm zebra")
		}
	})

	t.Run("no permissions denies all", func(t *testing.T) {
		created, _ := m.CreateKey(CreateKeyOptions{})
		result := m.CheckAccess(created.ID, Permission{Resource: "any", Action: "any"})
		if result.Allowed {
			t.Error("key with no permissions should deny all access")
		}
	})
}

func TestKeyCount(t *testing.T) {
	m := NewManager()

	if m.KeyCount() != 0 {
		t.Errorf("expected 0, got %d", m.KeyCount())
	}

	m.CreateKey(CreateKeyOptions{})
	if m.KeyCount() != 1 {
		t.Errorf("expected 1, got %d", m.KeyCount())
	}

	m.CreateKey(CreateKeyOptions{})
	m.CreateKey(CreateKeyOptions{})
	if m.KeyCount() != 3 {
		t.Errorf("expected 3, got %d", m.KeyCount())
	}
}
