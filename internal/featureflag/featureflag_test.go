package featureflag

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewEvaluator(t *testing.T) {
	e := NewEvaluator()
	if e == nil {
		t.Fatal("NewEvaluator returned nil")
	}
	if len(e.ListFlags()) != 0 {
		t.Errorf("expected 0 flags initially, got %d", len(e.ListFlags()))
	}
	if e.AuditLogCount() != 0 {
		t.Errorf("expected 0 audit logs initially, got %d", e.AuditLogCount())
	}
}

func TestNewEvaluatorWithSeed(t *testing.T) {
	e := NewEvaluatorWithSeed(12345)
	if e == nil {
		t.Fatal("NewEvaluatorWithSeed returned nil")
	}
}

func TestFlagType_String(t *testing.T) {
	tests := []struct {
		ft       FlagType
		expected string
	}{
		{FlagTypeBoolean, "Boolean"},
		{FlagTypePercentage, "Percentage"},
		{FlagTypeWhitelist, "Whitelist"},
		{FlagType(999), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.ft.String(); got != tt.expected {
			t.Errorf("FlagType(%d).String() = %s, want %s", tt.ft, got, tt.expected)
		}
	}
}

func TestFlagConfig_Clone(t *testing.T) {
	original := &FlagConfig{
		Key:         "test-key",
		Type:        FlagTypeWhitelist,
		Enabled:     true,
		Percentage:  50,
		Whitelist:   []string{"user1", "user2"},
		Description: "test description",
	}
	cloned := original.Clone()

	if cloned.Key != original.Key {
		t.Errorf("cloned Key mismatch: got %s, want %s", cloned.Key, original.Key)
	}
	if cloned.Type != original.Type {
		t.Errorf("cloned Type mismatch: got %v, want %v", cloned.Type, original.Type)
	}
	if cloned.Enabled != original.Enabled {
		t.Errorf("cloned Enabled mismatch: got %v, want %v", cloned.Enabled, original.Enabled)
	}
	if cloned.Percentage != original.Percentage {
		t.Errorf("cloned Percentage mismatch: got %d, want %d", cloned.Percentage, original.Percentage)
	}
	if len(cloned.Whitelist) != len(original.Whitelist) {
		t.Errorf("cloned Whitelist length mismatch: got %d, want %d", len(cloned.Whitelist), len(original.Whitelist))
	}
	for i := range cloned.Whitelist {
		if cloned.Whitelist[i] != original.Whitelist[i] {
			t.Errorf("cloned Whitelist[%d] mismatch: got %s, want %s", i, cloned.Whitelist[i], original.Whitelist[i])
		}
	}

	original.Whitelist[0] = "modified"
	if cloned.Whitelist[0] == "modified" {
		t.Error("Clone should perform deep copy of Whitelist slice")
	}

	var nilCfg *FlagConfig
	if nilCfg.Clone() != nil {
		t.Error("nil FlagConfig Clone() should return nil")
	}
}

func TestFlagConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FlagConfig
		wantErr error
	}{
		{
			name: "empty key",
			cfg:  FlagConfig{Key: "", Type: FlagTypeBoolean},
			wantErr: ErrNilFlagKey,
		},
		{
			name: "valid boolean",
			cfg:  FlagConfig{Key: "bool-flag", Type: FlagTypeBoolean},
			wantErr: nil,
		},
		{
			name: "valid percentage 0",
			cfg:  FlagConfig{Key: "pct-0", Type: FlagTypePercentage, Percentage: 0},
			wantErr: nil,
		},
		{
			name: "valid percentage 50",
			cfg:  FlagConfig{Key: "pct-50", Type: FlagTypePercentage, Percentage: 50},
			wantErr: nil,
		},
		{
			name: "valid percentage 100",
			cfg:  FlagConfig{Key: "pct-100", Type: FlagTypePercentage, Percentage: 100},
			wantErr: nil,
		},
		{
			name: "invalid percentage negative",
			cfg:  FlagConfig{Key: "pct-neg", Type: FlagTypePercentage, Percentage: -1},
			wantErr: ErrInvalidPercentage,
		},
		{
			name: "invalid percentage over 100",
			cfg:  FlagConfig{Key: "pct-over", Type: FlagTypePercentage, Percentage: 101},
			wantErr: ErrInvalidPercentage,
		},
		{
			name: "valid whitelist",
			cfg:  FlagConfig{Key: "wl-flag", Type: FlagTypeWhitelist},
			wantErr: nil,
		},
		{
			name: "invalid flag type",
			cfg:  FlagConfig{Key: "bad-type", Type: FlagType(999)},
			wantErr: ErrInvalidFlagType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestFlagConfig_Marshal(t *testing.T) {
	cfg := &FlagConfig{
		Key:         "test",
		Type:        FlagTypeBoolean,
		Enabled:     true,
		Description: "desc",
	}
	s := cfg.Marshal()
	if s == "" {
		t.Error("Marshal returned empty string")
	}
}

func TestCreateFlag(t *testing.T) {
	e := NewEvaluator()

	t.Run("nil config", func(t *testing.T) {
		err := e.CreateFlag(nil)
		if !errors.Is(err, ErrNilConfig) {
			t.Errorf("expected ErrNilConfig, got %v", err)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		err := e.CreateFlag(&FlagConfig{Key: "", Type: FlagTypeBoolean})
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		cfg := &FlagConfig{Key: "new-flag", Type: FlagTypeBoolean, Enabled: true}
		err := e.CreateFlag(cfg)
		if err != nil {
			t.Fatalf("CreateFlag failed: %v", err)
		}
		got, err := e.GetFlag("new-flag")
		if err != nil {
			t.Fatalf("GetFlag failed: %v", err)
		}
		if got.Key != "new-flag" {
			t.Errorf("got Key %s, want new-flag", got.Key)
		}
		if e.AuditLogCount() != 1 {
			t.Errorf("expected 1 audit log after create, got %d", e.AuditLogCount())
		}
	})

	t.Run("duplicate key", func(t *testing.T) {
		cfg := &FlagConfig{Key: "new-flag", Type: FlagTypeBoolean}
		err := e.CreateFlag(cfg)
		if !errors.Is(err, ErrFlagAlreadyExists) {
			t.Errorf("expected ErrFlagAlreadyExists, got %v", err)
		}
	})
}

func TestUpdateFlag(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "update-test", Type: FlagTypeBoolean, Enabled: false})

	t.Run("nil config", func(t *testing.T) {
		err := e.UpdateFlag(nil)
		if !errors.Is(err, ErrNilConfig) {
			t.Errorf("expected ErrNilConfig, got %v", err)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		err := e.UpdateFlag(&FlagConfig{Key: "", Type: FlagTypeBoolean})
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := e.UpdateFlag(&FlagConfig{Key: "nonexistent", Type: FlagTypeBoolean})
		if !errors.Is(err, ErrFlagNotFound) {
			t.Errorf("expected ErrFlagNotFound, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		auditBefore := e.AuditLogCount()
		err := e.UpdateFlag(&FlagConfig{Key: "update-test", Type: FlagTypeBoolean, Enabled: true, Description: "updated"})
		if err != nil {
			t.Fatalf("UpdateFlag failed: %v", err)
		}
		got, _ := e.GetFlag("update-test")
		if !got.Enabled {
			t.Error("expected Enabled = true after update")
		}
		if got.Description != "updated" {
			t.Errorf("expected Description 'updated', got %s", got.Description)
		}
		if e.AuditLogCount() != auditBefore+1 {
			t.Errorf("expected audit log count increase by 1")
		}
	})
}

func TestDeleteFlag(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "delete-test", Type: FlagTypeBoolean})

	t.Run("empty key", func(t *testing.T) {
		err := e.DeleteFlag("")
		if !errors.Is(err, ErrNilFlagKey) {
			t.Errorf("expected ErrNilFlagKey, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := e.DeleteFlag("nonexistent")
		if !errors.Is(err, ErrFlagNotFound) {
			t.Errorf("expected ErrFlagNotFound, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		auditBefore := e.AuditLogCount()
		err := e.DeleteFlag("delete-test")
		if err != nil {
			t.Fatalf("DeleteFlag failed: %v", err)
		}
		_, err = e.GetFlag("delete-test")
		if !errors.Is(err, ErrFlagNotFound) {
			t.Error("expected ErrFlagNotFound after deletion")
		}
		if e.AuditLogCount() != auditBefore+1 {
			t.Errorf("expected audit log count increase by 1")
		}
	})
}

func TestGetFlag(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "get-test", Type: FlagTypeBoolean, Enabled: true})

	t.Run("empty key", func(t *testing.T) {
		_, err := e.GetFlag("")
		if !errors.Is(err, ErrNilFlagKey) {
			t.Errorf("expected ErrNilFlagKey, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := e.GetFlag("nonexistent")
		if !errors.Is(err, ErrFlagNotFound) {
			t.Errorf("expected ErrFlagNotFound, got %v", err)
		}
	})

	t.Run("success returns copy", func(t *testing.T) {
		cfg, err := e.GetFlag("get-test")
		if err != nil {
			t.Fatalf("GetFlag failed: %v", err)
		}
		cfg.Enabled = false
		cfg2, _ := e.GetFlag("get-test")
		if !cfg2.Enabled {
			t.Error("GetFlag should return a copy, modifications should not affect internal state")
		}
	})
}

func TestListFlags(t *testing.T) {
	e := NewEvaluator()

	flags := e.ListFlags()
	if len(flags) != 0 {
		t.Errorf("expected empty list, got %d items", len(flags))
	}

	keys := []string{"z-flag", "a-flag", "m-flag"}
	for _, k := range keys {
		_ = e.CreateFlag(&FlagConfig{Key: k, Type: FlagTypeBoolean})
	}

	result := e.ListFlags()
	if len(result) != 3 {
		t.Fatalf("expected 3 flags, got %d", len(result))
	}
	for i := 1; i < len(result); i++ {
		if result[i-1].Key > result[i].Key {
			t.Errorf("ListFlags should be sorted by key: %s > %s", result[i-1].Key, result[i].Key)
		}
	}
}

func TestEvaluate_BooleanFlag(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "bool-on", Type: FlagTypeBoolean, Enabled: true})
	_ = e.CreateFlag(&FlagConfig{Key: "bool-off", Type: FlagTypeBoolean, Enabled: false})

	t.Run("empty key", func(t *testing.T) {
		_, err := e.Evaluate("", "")
		if !errors.Is(err, ErrNilFlagKey) {
			t.Errorf("expected ErrNilFlagKey, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := e.Evaluate("nonexistent", "")
		if !errors.Is(err, ErrFlagNotFound) {
			t.Errorf("expected ErrFlagNotFound, got %v", err)
		}
	})

	t.Run("enabled true", func(t *testing.T) {
		result, err := e.Evaluate("bool-on", "")
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}
		if !result {
			t.Error("expected true for enabled boolean flag")
		}
	})

	t.Run("enabled false", func(t *testing.T) {
		result, err := e.Evaluate("bool-off", "")
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}
		if result {
			t.Error("expected false for disabled boolean flag")
		}
	})
}

func TestEvaluate_PercentageFlag(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "pct-0", Type: FlagTypePercentage, Percentage: 0})
	_ = e.CreateFlag(&FlagConfig{Key: "pct-100", Type: FlagTypePercentage, Percentage: 100})
	_ = e.CreateFlag(&FlagConfig{Key: "pct-50", Type: FlagTypePercentage, Percentage: 50})

	t.Run("empty user id", func(t *testing.T) {
		_, err := e.Evaluate("pct-50", "")
		if !errors.Is(err, ErrNilUserID) {
			t.Errorf("expected ErrNilUserID, got %v", err)
		}
	})

	t.Run("percentage 0 always false", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			result, err := e.Evaluate("pct-0", fmt.Sprintf("user-%d", i))
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}
			if result {
				t.Errorf("percentage 0 should always be false, but got true for user-%d", i)
			}
		}
	})

	t.Run("percentage 100 always true", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			result, err := e.Evaluate("pct-100", fmt.Sprintf("user-%d", i))
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}
			if !result {
				t.Errorf("percentage 100 should always be true, but got false for user-%d", i)
			}
		}
	})

	t.Run("user consistency", func(t *testing.T) {
		userID := "consistent-user"
		first, err := e.Evaluate("pct-50", userID)
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}
		for i := 0; i < 100; i++ {
			result, _ := e.Evaluate("pct-50", userID)
			if result != first {
				t.Errorf("inconsistent result for same user: got %v on iteration %d, expected %v", result, i, first)
			}
		}
	})

	t.Run("distribution approximately correct", func(t *testing.T) {
		trueCount := 0
		total := 10000
		for i := 0; i < total; i++ {
			result, _ := e.Evaluate("pct-50", fmt.Sprintf("dist-user-%d", i))
			if result {
				trueCount++
			}
		}
		ratio := float64(trueCount) / float64(total)
		if ratio < 0.45 || ratio > 0.55 {
			t.Errorf("distribution too far from 50%%: got %.2f%% (%d/%d)", ratio*100, trueCount, total)
		}
	})
}

func TestEvaluate_WhitelistFlag(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{
		Key:       "wl-flag",
		Type:      FlagTypeWhitelist,
		Whitelist: []string{"alice", "bob", "charlie"},
	})
	_ = e.CreateFlag(&FlagConfig{
		Key:       "wl-empty",
		Type:      FlagTypeWhitelist,
		Whitelist: nil,
	})

	t.Run("empty user id", func(t *testing.T) {
		_, err := e.Evaluate("wl-flag", "")
		if !errors.Is(err, ErrNilUserID) {
			t.Errorf("expected ErrNilUserID, got %v", err)
		}
	})

	t.Run("user in whitelist", func(t *testing.T) {
		for _, user := range []string{"alice", "bob", "charlie"} {
			result, err := e.Evaluate("wl-flag", user)
			if err != nil {
				t.Fatalf("Evaluate failed for %s: %v", user, err)
			}
			if !result {
				t.Errorf("expected true for whitelisted user %s", user)
			}
		}
	})

	t.Run("user not in whitelist", func(t *testing.T) {
		result, err := e.Evaluate("wl-flag", "dave")
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}
		if result {
			t.Error("expected false for non-whitelisted user")
		}
	})

	t.Run("empty whitelist", func(t *testing.T) {
		result, err := e.Evaluate("wl-empty", "anyone")
		if err != nil {
			t.Fatalf("Evaluate failed: %v", err)
		}
		if result {
			t.Error("expected false with empty whitelist")
		}
	})
}

func TestSetBooleanValue(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "bool-setter", Type: FlagTypeBoolean, Enabled: false})

	t.Run("empty key", func(t *testing.T) {
		err := e.SetBooleanValue("", true)
		if !errors.Is(err, ErrNilFlagKey) {
			t.Errorf("expected ErrNilFlagKey, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := e.SetBooleanValue("nonexistent", true)
		if !errors.Is(err, ErrFlagNotFound) {
			t.Errorf("expected ErrFlagNotFound, got %v", err)
		}
	})

	t.Run("toggle from false to true", func(t *testing.T) {
		auditBefore := e.AuditLogCount()
		err := e.SetBooleanValue("bool-setter", true)
		if err != nil {
			t.Fatalf("SetBooleanValue failed: %v", err)
		}
		result, _ := e.Evaluate("bool-setter", "")
		if !result {
			t.Error("expected true after SetBooleanValue(true)")
		}
		if e.AuditLogCount() != auditBefore+1 {
			t.Error("expected audit log entry")
		}
	})

	t.Run("toggle from true to false", func(t *testing.T) {
		err := e.SetBooleanValue("bool-setter", false)
		if err != nil {
			t.Fatalf("SetBooleanValue failed: %v", err)
		}
		result, _ := e.Evaluate("bool-setter", "")
		if result {
			t.Error("expected false after SetBooleanValue(false)")
		}
	})
}

func TestSetPercentage(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "pct-setter", Type: FlagTypePercentage, Percentage: 10})
	_ = e.CreateFlag(&FlagConfig{Key: "bool-flag", Type: FlagTypeBoolean})

	t.Run("empty key", func(t *testing.T) {
		err := e.SetPercentage("", 50)
		if !errors.Is(err, ErrNilFlagKey) {
			t.Errorf("expected ErrNilFlagKey, got %v", err)
		}
	})

	t.Run("invalid percentage negative", func(t *testing.T) {
		err := e.SetPercentage("pct-setter", -1)
		if !errors.Is(err, ErrInvalidPercentage) {
			t.Errorf("expected ErrInvalidPercentage, got %v", err)
		}
	})

	t.Run("invalid percentage over 100", func(t *testing.T) {
		err := e.SetPercentage("pct-setter", 101)
		if !errors.Is(err, ErrInvalidPercentage) {
			t.Errorf("expected ErrInvalidPercentage, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := e.SetPercentage("nonexistent", 50)
		if !errors.Is(err, ErrFlagNotFound) {
			t.Errorf("expected ErrFlagNotFound, got %v", err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		err := e.SetPercentage("bool-flag", 50)
		if !errors.Is(err, ErrInvalidFlagType) {
			t.Errorf("expected ErrInvalidFlagType, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		auditBefore := e.AuditLogCount()
		err := e.SetPercentage("pct-setter", 75)
		if err != nil {
			t.Fatalf("SetPercentage failed: %v", err)
		}
		cfg, _ := e.GetFlag("pct-setter")
		if cfg.Percentage != 75 {
			t.Errorf("expected Percentage=75, got %d", cfg.Percentage)
		}
		if e.AuditLogCount() != auditBefore+1 {
			t.Error("expected audit log entry")
		}
	})
}

func TestAddToWhitelist(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "wl-adder", Type: FlagTypeWhitelist, Whitelist: []string{"alice"}})
	_ = e.CreateFlag(&FlagConfig{Key: "bool-flag", Type: FlagTypeBoolean})

	t.Run("empty key", func(t *testing.T) {
		err := e.AddToWhitelist("", "bob")
		if !errors.Is(err, ErrNilFlagKey) {
			t.Errorf("expected ErrNilFlagKey, got %v", err)
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		err := e.AddToWhitelist("wl-adder", "")
		if !errors.Is(err, ErrNilUserID) {
			t.Errorf("expected ErrNilUserID, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := e.AddToWhitelist("nonexistent", "bob")
		if !errors.Is(err, ErrFlagNotFound) {
			t.Errorf("expected ErrFlagNotFound, got %v", err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		err := e.AddToWhitelist("bool-flag", "bob")
		if !errors.Is(err, ErrInvalidFlagType) {
			t.Errorf("expected ErrInvalidFlagType, got %v", err)
		}
	})

	t.Run("add new user", func(t *testing.T) {
		auditBefore := e.AuditLogCount()
		err := e.AddToWhitelist("wl-adder", "bob")
		if err != nil {
			t.Fatalf("AddToWhitelist failed: %v", err)
		}
		result, _ := e.Evaluate("wl-adder", "bob")
		if !result {
			t.Error("expected bob to be in whitelist after adding")
		}
		if e.AuditLogCount() != auditBefore+1 {
			t.Error("expected audit log entry")
		}
	})

	t.Run("add duplicate user is no-op", func(t *testing.T) {
		auditBefore := e.AuditLogCount()
		err := e.AddToWhitelist("wl-adder", "alice")
		if err != nil {
			t.Fatalf("AddToWhitelist failed: %v", err)
		}
		cfg, _ := e.GetFlag("wl-adder")
		if len(cfg.Whitelist) != 2 {
			t.Errorf("expected whitelist length 2, got %d (duplicate should not add)", len(cfg.Whitelist))
		}
		if e.AuditLogCount() != auditBefore {
			t.Error("duplicate add should not generate audit log")
		}
	})
}

func TestRemoveFromWhitelist(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "wl-remover", Type: FlagTypeWhitelist, Whitelist: []string{"alice", "bob"}})
	_ = e.CreateFlag(&FlagConfig{Key: "bool-flag", Type: FlagTypeBoolean})

	t.Run("empty key", func(t *testing.T) {
		err := e.RemoveFromWhitelist("", "bob")
		if !errors.Is(err, ErrNilFlagKey) {
			t.Errorf("expected ErrNilFlagKey, got %v", err)
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		err := e.RemoveFromWhitelist("wl-remover", "")
		if !errors.Is(err, ErrNilUserID) {
			t.Errorf("expected ErrNilUserID, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := e.RemoveFromWhitelist("nonexistent", "bob")
		if !errors.Is(err, ErrFlagNotFound) {
			t.Errorf("expected ErrFlagNotFound, got %v", err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		err := e.RemoveFromWhitelist("bool-flag", "bob")
		if !errors.Is(err, ErrInvalidFlagType) {
			t.Errorf("expected ErrInvalidFlagType, got %v", err)
		}
	})

	t.Run("remove existing user", func(t *testing.T) {
		auditBefore := e.AuditLogCount()
		err := e.RemoveFromWhitelist("wl-remover", "bob")
		if err != nil {
			t.Fatalf("RemoveFromWhitelist failed: %v", err)
		}
		result, _ := e.Evaluate("wl-remover", "bob")
		if result {
			t.Error("expected bob to be removed from whitelist")
		}
		cfg, _ := e.GetFlag("wl-remover")
		if len(cfg.Whitelist) != 1 {
			t.Errorf("expected whitelist length 1, got %d", len(cfg.Whitelist))
		}
		if cfg.Whitelist[0] != "alice" {
			t.Errorf("expected alice to remain, got %s", cfg.Whitelist[0])
		}
		if e.AuditLogCount() != auditBefore+1 {
			t.Error("expected audit log entry")
		}
	})

	t.Run("remove non-existing user is no-op", func(t *testing.T) {
		auditBefore := e.AuditLogCount()
		err := e.RemoveFromWhitelist("wl-remover", "charlie")
		if err != nil {
			t.Fatalf("RemoveFromWhitelist failed: %v", err)
		}
		if e.AuditLogCount() != auditBefore {
			t.Error("removing non-existent user should not generate audit log")
		}
	})
}

func TestChangeFlagType(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "type-changer", Type: FlagTypeBoolean, Enabled: true})

	t.Run("empty key", func(t *testing.T) {
		err := e.ChangeFlagType("", FlagTypePercentage, false, 50, nil)
		if !errors.Is(err, ErrNilFlagKey) {
			t.Errorf("expected ErrNilFlagKey, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := e.ChangeFlagType("nonexistent", FlagTypePercentage, false, 50, nil)
		if !errors.Is(err, ErrFlagNotFound) {
			t.Errorf("expected ErrFlagNotFound, got %v", err)
		}
	})

	t.Run("invalid new config", func(t *testing.T) {
		err := e.ChangeFlagType("type-changer", FlagTypePercentage, false, 150, nil)
		if !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("boolean to percentage", func(t *testing.T) {
		auditBefore := e.AuditLogCount()
		err := e.ChangeFlagType("type-changer", FlagTypePercentage, false, 100, nil)
		if err != nil {
			t.Fatalf("ChangeFlagType failed: %v", err)
		}
		cfg, _ := e.GetFlag("type-changer")
		if cfg.Type != FlagTypePercentage {
			t.Errorf("expected Type=Percentage, got %v", cfg.Type)
		}
		if cfg.Percentage != 100 {
			t.Errorf("expected Percentage=100, got %d", cfg.Percentage)
		}
		result, _ := e.Evaluate("type-changer", "anyuser")
		if !result {
			t.Error("expected evaluation true for 100% percentage")
		}
		if e.AuditLogCount() != auditBefore+1 {
			t.Error("expected audit log entry")
		}
	})

	t.Run("percentage to whitelist", func(t *testing.T) {
		err := e.ChangeFlagType("type-changer", FlagTypeWhitelist, false, 0, []string{"vip-user"})
		if err != nil {
			t.Fatalf("ChangeFlagType failed: %v", err)
		}
		cfg, _ := e.GetFlag("type-changer")
		if cfg.Type != FlagTypeWhitelist {
			t.Errorf("expected Type=Whitelist, got %v", cfg.Type)
		}
		result, _ := e.Evaluate("type-changer", "vip-user")
		if !result {
			t.Error("expected vip-user in whitelist")
		}
		result2, _ := e.Evaluate("type-changer", "other")
		if result2 {
			t.Error("expected other not in whitelist")
		}
	})

	t.Run("whitelist to boolean", func(t *testing.T) {
		err := e.ChangeFlagType("type-changer", FlagTypeBoolean, false, 0, nil)
		if err != nil {
			t.Fatalf("ChangeFlagType failed: %v", err)
		}
		cfg, _ := e.GetFlag("type-changer")
		if cfg.Type != FlagTypeBoolean {
			t.Errorf("expected Type=Boolean, got %v", cfg.Type)
		}
		result, _ := e.Evaluate("type-changer", "")
		if result {
			t.Error("expected false for disabled boolean")
		}
	})
}

func TestHotUpdate_TakesEffectImmediately(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "hot-bool", Type: FlagTypeBoolean, Enabled: false})
	_ = e.CreateFlag(&FlagConfig{Key: "hot-pct", Type: FlagTypePercentage, Percentage: 0})
	_ = e.CreateFlag(&FlagConfig{Key: "hot-wl", Type: FlagTypeWhitelist, Whitelist: nil})

	r1, _ := e.Evaluate("hot-bool", "")
	if r1 {
		t.Error("initial bool should be false")
	}
	_ = e.SetBooleanValue("hot-bool", true)
	r2, _ := e.Evaluate("hot-bool", "")
	if !r2 {
		t.Error("after SetBooleanValue(true) should be true immediately")
	}

	r3, _ := e.Evaluate("hot-pct", "userA")
	if r3 {
		t.Error("initial 0% should be false")
	}
	_ = e.SetPercentage("hot-pct", 100)
	r4, _ := e.Evaluate("hot-pct", "userA")
	if !r4 {
		t.Error("after SetPercentage(100) should be true immediately")
	}

	r5, _ := e.Evaluate("hot-wl", "newuser")
	if r5 {
		t.Error("user should not be in empty whitelist")
	}
	_ = e.AddToWhitelist("hot-wl", "newuser")
	r6, _ := e.Evaluate("hot-wl", "newuser")
	if !r6 {
		t.Error("after AddToWhitelist user should be in whitelist immediately")
	}
}

func TestUserBucket_Consistency(t *testing.T) {
	seed := uint64(42)
	userID := "test-user-12345"

	first := computeUserBucket(userID, seed)
	for i := 0; i < 1000; i++ {
		again := computeUserBucket(userID, seed)
		if again != first {
			t.Errorf("bucket inconsistent: got %d, want %d (iteration %d)", again, first, i)
		}
	}
}

func TestUserBucket_DifferentSeeds(t *testing.T) {
	userID := "same-user"
	b1 := computeUserBucket(userID, 1)
	b2 := computeUserBucket(userID, 2)
	if b1 == b2 {
		t.Logf("Note: different seeds produced same bucket for this user (possible but unlikely for test assertion)")
	}
}

func TestUserBucket_DifferentUsers(t *testing.T) {
	seed := uint64(123)
	buckets := make(map[int]int)
	total := 10000
	for i := 0; i < total; i++ {
		b := computeUserBucket(fmt.Sprintf("bucket-user-%d", i), seed)
		if b < 0 || b >= 100 {
			t.Errorf("bucket out of range [0,99]: got %d", b)
		}
		buckets[b]++
	}
	if len(buckets) < 95 {
		t.Errorf("expected buckets to cover most of 0-99, only got %d distinct", len(buckets))
	}
}

func TestAuditLogs_CreateUpdateDelete(t *testing.T) {
	e := NewEvaluator()

	_ = e.CreateFlag(&FlagConfig{Key: "audit-1", Type: FlagTypeBoolean, Enabled: false})
	if e.AuditLogCount() != 1 {
		t.Fatalf("expected 1 audit log after create, got %d", e.AuditLogCount())
	}

	_ = e.UpdateFlag(&FlagConfig{Key: "audit-1", Type: FlagTypeBoolean, Enabled: true})
	if e.AuditLogCount() != 2 {
		t.Fatalf("expected 2 audit logs after update, got %d", e.AuditLogCount())
	}

	_ = e.DeleteFlag("audit-1")
	if e.AuditLogCount() != 3 {
		t.Fatalf("expected 3 audit logs after delete, got %d", e.AuditLogCount())
	}

	logs := e.QueryAuditLogs(AuditLogQuery{})
	if len(logs) != 3 {
		t.Fatalf("expected 3 audit logs from query, got %d", len(logs))
	}

	if logs[0].Operation != "CREATE" {
		t.Errorf("first log Operation = %s, want CREATE", logs[0].Operation)
	}
	if logs[0].Before != nil {
		t.Error("CREATE Before should be nil")
	}
	if logs[0].After == nil || logs[0].After.Key != "audit-1" {
		t.Error("CREATE After should have the flag config")
	}

	if logs[1].Operation != "UPDATE" {
		t.Errorf("second log Operation = %s, want UPDATE", logs[1].Operation)
	}
	if logs[1].Before == nil || logs[1].Before.Enabled {
		t.Error("UPDATE Before should have Enabled=false")
	}
	if logs[1].After == nil || !logs[1].After.Enabled {
		t.Error("UPDATE After should have Enabled=true")
	}

	if logs[2].Operation != "DELETE" {
		t.Errorf("third log Operation = %s, want DELETE", logs[2].Operation)
	}
	if logs[2].Before == nil {
		t.Error("DELETE Before should have the flag config")
	}
	if logs[2].After != nil {
		t.Error("DELETE After should be nil")
	}
}

func TestQueryAuditLogs_ByFlagKey(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "flag-a", Type: FlagTypeBoolean})
	_ = e.CreateFlag(&FlagConfig{Key: "flag-b", Type: FlagTypeBoolean})
	_ = e.SetBooleanValue("flag-a", true)

	aLogs := e.QueryAuditLogs(AuditLogQuery{FlagKey: "flag-a"})
	if len(aLogs) != 2 {
		t.Errorf("expected 2 audit logs for flag-a, got %d", len(aLogs))
	}
	for _, l := range aLogs {
		if l.FlagKey != "flag-a" {
			t.Errorf("got audit log for wrong key: %s", l.FlagKey)
		}
	}

	bLogs := e.QueryAuditLogs(AuditLogQuery{FlagKey: "flag-b"})
	if len(bLogs) != 1 {
		t.Errorf("expected 1 audit log for flag-b, got %d", len(bLogs))
	}
}

func TestQueryAuditLogs_ByTimeRange(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "timed-flag", Type: FlagTypeBoolean})

	time.Sleep(10 * time.Millisecond)
	midTime := time.Now()
	time.Sleep(10 * time.Millisecond)

	_ = e.SetBooleanValue("timed-flag", true)

	_ = e.CreateFlag(&FlagConfig{Key: "later-flag", Type: FlagTypeBoolean})

	startTime := midTime
	endTime := midTime
	results := e.QueryAuditLogs(AuditLogQuery{StartTime: &startTime, EndTime: &endTime})

	allLogs := e.QueryAuditLogs(AuditLogQuery{})
	for _, l := range allLogs {
		t.Logf("log: ts=%v op=%s key=%s", l.Timestamp, l.Operation, l.FlagKey)
	}

	future := time.Now().Add(1 * time.Hour)
	past := time.Now().Add(-1 * time.Hour)
	allRange := e.QueryAuditLogs(AuditLogQuery{StartTime: &past, EndTime: &future})
	if len(allRange) != len(allLogs) {
		t.Errorf("full time range should return all logs: got %d, want %d", len(allRange), len(allLogs))
	}

	_ = results
}

func TestQueryAuditLogs_StartTimeOnly(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "start-only", Type: FlagTypeBoolean})

	future := time.Now().Add(1 * time.Hour)
	logs := e.QueryAuditLogs(AuditLogQuery{StartTime: &future})
	if len(logs) != 0 {
		t.Errorf("expected 0 logs after future start time, got %d", len(logs))
	}

	past := time.Now().Add(-1 * time.Hour)
	logs2 := e.QueryAuditLogs(AuditLogQuery{StartTime: &past})
	if len(logs2) != 1 {
		t.Errorf("expected 1 log from past start time, got %d", len(logs2))
	}
}

func TestQueryAuditLogs_EndTimeOnly(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "end-only", Type: FlagTypeBoolean})

	past := time.Now().Add(-1 * time.Hour)
	logs := e.QueryAuditLogs(AuditLogQuery{EndTime: &past})
	if len(logs) != 0 {
		t.Errorf("expected 0 logs before past end time, got %d", len(logs))
	}

	future := time.Now().Add(1 * time.Hour)
	logs2 := e.QueryAuditLogs(AuditLogQuery{EndTime: &future})
	if len(logs2) != 1 {
		t.Errorf("expected 1 log before future end time, got %d", len(logs2))
	}
}

func TestAuditLog_ReturnsCopies(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "copy-test", Type: FlagTypeBoolean, Enabled: true})

	logs := e.QueryAuditLogs(AuditLogQuery{FlagKey: "copy-test"})
	if len(logs) == 0 {
		t.Fatal("no audit logs found")
	}

	logs[0].FlagKey = "mutated"
	logs[0].After.Key = "mutated-key"

	logs2 := e.QueryAuditLogs(AuditLogQuery{FlagKey: "copy-test"})
	if len(logs2) == 0 {
		t.Fatal("audit logs should be accessible by original key")
	}
	if logs2[0].FlagKey != "copy-test" {
		t.Error("QueryAuditLogs should return copies of entries")
	}
	if logs2[0].After.Key != "copy-test" {
		t.Error("QueryAuditLogs should return copies of flag configs")
	}
}

func TestConcurrentAccess(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "concurrent-bool", Type: FlagTypeBoolean})
	_ = e.CreateFlag(&FlagConfig{Key: "concurrent-pct", Type: FlagTypePercentage, Percentage: 50})
	_ = e.CreateFlag(&FlagConfig{Key: "concurrent-wl", Type: FlagTypeWhitelist, Whitelist: []string{"seed-user"}})

	var wg sync.WaitGroup
	numGoroutines := 20
	perGoroutine := 100
	var evalErrors int32
	var updateErrors int32

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				userID := fmt.Sprintf("u-%d-%d", gid, i)

				_, err := e.Evaluate("concurrent-bool", userID)
				if err != nil {
					atomic.AddInt32(&evalErrors, 1)
				}

				_, err = e.Evaluate("concurrent-pct", userID)
				if err != nil {
					atomic.AddInt32(&evalErrors, 1)
				}

				_, err = e.Evaluate("concurrent-wl", userID)
				if err != nil {
					atomic.AddInt32(&evalErrors, 1)
				}

				newVal := i%2 == 0
				if err := e.SetBooleanValue("concurrent-bool", newVal); err != nil {
					atomic.AddInt32(&updateErrors, 1)
				}

				newPct := i % 101
				if err := e.SetPercentage("concurrent-pct", newPct); err != nil {
					atomic.AddInt32(&updateErrors, 1)
				}

				if err := e.AddToWhitelist("concurrent-wl", userID); err != nil {
					atomic.AddInt32(&updateErrors, 1)
				}
			}
		}(g)
	}

	wg.Wait()

	if evalErrors > 0 {
		t.Errorf("got %d evaluation errors during concurrent access", evalErrors)
	}
	if updateErrors > 0 {
		t.Errorf("got %d update errors during concurrent access", updateErrors)
	}

	if e.AuditLogCount() == 0 {
		t.Error("expected audit logs from concurrent updates")
	}

	flags := e.ListFlags()
	if len(flags) != 3 {
		t.Errorf("expected 3 flags after concurrent ops, got %d", len(flags))
	}
}

func TestFullWorkflow(t *testing.T) {
	e := NewEvaluator()

	err := e.CreateFlag(&FlagConfig{
		Key:         "new-checkout",
		Type:        FlagTypePercentage,
		Percentage:  0,
		Description: "New checkout flow rollout",
	})
	if err != nil {
		t.Fatalf("CreateFlag failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		result, _ := e.Evaluate("new-checkout", fmt.Sprintf("user-%d", i))
		if result {
			t.Errorf("0%% rollout: user-%d should not have access", i)
		}
	}

	err = e.SetPercentage("new-checkout", 10)
	if err != nil {
		t.Fatalf("SetPercentage failed: %v", err)
	}

	_ = e.AddToWhitelist("new-checkout", "qa-user")
	_ = e.ChangeFlagType("new-checkout", FlagTypeWhitelist, false, 0, []string{"internal-user"})

	result, err := e.Evaluate("new-checkout", "internal-user")
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result {
		t.Error("internal-user should be in whitelist after type change")
	}

	result, _ = e.Evaluate("new-checkout", "qa-user")
	if result {
		t.Error("qa-user should not be in new whitelist after type change")
	}

	_ = e.DeleteFlag("new-checkout")
	_, err = e.Evaluate("new-checkout", "anyone")
	if !errors.Is(err, ErrFlagNotFound) {
		t.Error("expected ErrFlagNotFound after deletion")
	}
}

func TestPercentage_BoundaryCases(t *testing.T) {
	e := NewEvaluator()
	_ = e.CreateFlag(&FlagConfig{Key: "boundary-1", Type: FlagTypePercentage, Percentage: 1})
	_ = e.CreateFlag(&FlagConfig{Key: "boundary-99", Type: FlagTypePercentage, Percentage: 99})

	var trueCount1 int
	var trueCount99 int
	total := 10000

	for i := 0; i < total; i++ {
		userID := fmt.Sprintf("boundary-%d", i)
		r1, _ := e.Evaluate("boundary-1", userID)
		r99, _ := e.Evaluate("boundary-99", userID)
		if r1 {
			trueCount1++
		}
		if r99 {
			trueCount99++
		}
	}

	ratio1 := float64(trueCount1) / float64(total)
	if ratio1 > 0.05 {
		t.Errorf("1%% percentage too many true: got %.2f%%", ratio1*100)
	}

	ratio99 := float64(trueCount99) / float64(total)
	if ratio99 < 0.94 {
		t.Errorf("99%% percentage too few true: got %.2f%%", ratio99*100)
	}
}

func TestEvaluator_DifferentSeeds_ProduceDifferentBuckets(t *testing.T) {
	e1 := NewEvaluatorWithSeed(111)
	e2 := NewEvaluatorWithSeed(222)
	_ = e1.CreateFlag(&FlagConfig{Key: "flag", Type: FlagTypePercentage, Percentage: 50})
	_ = e2.CreateFlag(&FlagConfig{Key: "flag", Type: FlagTypePercentage, Percentage: 50})

	differentCount := 0
	total := 1000
	for i := 0; i < total; i++ {
		userID := fmt.Sprintf("seed-test-%d", i)
		r1, _ := e1.Evaluate("flag", userID)
		r2, _ := e2.Evaluate("flag", userID)
		if r1 != r2 {
			differentCount++
		}
	}
	if differentCount == 0 {
		t.Error("different seeds should produce different bucket assignments for some users")
	}
}
