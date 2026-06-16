package passpolicy

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	validPassword = "Abcdefg1!"
)

func TestNewEngine(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
	if engine.UserCount() != 0 {
		t.Errorf("expected 0 users, got %d", engine.UserCount())
	}
	cfg := engine.GetConfig()
	if cfg.MinLength != DefaultMinLength {
		t.Errorf("expected default MinLength %d, got %d", DefaultMinLength, cfg.MinLength)
	}
}

func TestNewEngineWithConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := Config{
			MinLength:    12,
			BcryptCost: 12,
			HistoryDepth: 10,
			ExpiryDays: 60,
			WarningDays: 5,
			Complexity: ComplexityConfig{
				RequireUppercase: true,
				RequireLowercase: true,
				RequireDigit:     true,
				RequireSpecial:  false,
			},
		}
		engine, err := NewEngineWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewEngineWithConfig failed: %v", err)
		}
		gotCfg := engine.GetConfig()
		if gotCfg.MinLength != 12 {
			t.Errorf("expected MinLength 12, got %d", gotCfg.MinLength)
		}
		if gotCfg.Complexity.RequireSpecial {
			t.Error("expected RequireSpecial false")
		}
	})

	t.Run("invalid min length", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 0
		_, err := NewEngineWithConfig(cfg)
		if !errors.Is(err, ErrInvalidMinLength) {
			t.Errorf("expected ErrInvalidMinLength, got %v", err)
		}
	})

	t.Run("invalid bcrypt cost too low", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.BcryptCost = MinBcryptCost - 1
		_, err := NewEngineWithConfig(cfg)
		if !errors.Is(err, ErrInvalidBcryptCost) {
			t.Errorf("expected ErrInvalidBcryptCost, got %v", err)
		}
	})

	t.Run("invalid bcrypt cost too high", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.BcryptCost = MaxBcryptCost + 1
		_, err := NewEngineWithConfig(cfg)
		if !errors.Is(err, ErrInvalidBcryptCost) {
			t.Errorf("expected ErrInvalidBcryptCost, got %v", err)
		}
	})

	t.Run("invalid history depth", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.HistoryDepth = -1
		_, err := NewEngineWithConfig(cfg)
		if !errors.Is(err, ErrInvalidHistoryDepth) {
			t.Errorf("expected ErrInvalidHistoryDepth, got %v", err)
		}
	})

	t.Run("invalid expiry days", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ExpiryDays = -1
		_, err := NewEngineWithConfig(cfg)
		if !errors.Is(err, ErrInvalidExpiryDays) {
			t.Errorf("expected ErrInvalidExpiryDays, got %v", err)
		}
	})

	t.Run("invalid warning days", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.WarningDays = -1
		_, err := NewEngineWithConfig(cfg)
		if !errors.Is(err, ErrInvalidWarningDays) {
			t.Errorf("expected ErrInvalidWarningDays, got %v", err)
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MinLength != DefaultMinLength {
		t.Errorf("expected %d, got %d", DefaultMinLength, cfg.MinLength)
	}
	if cfg.BcryptCost != DefaultBcryptCost {
		t.Errorf("expected %d, got %d", DefaultBcryptCost, cfg.BcryptCost)
	}
	if cfg.HistoryDepth != DefaultHistoryDepth {
		t.Errorf("expected %d, got %d", DefaultHistoryDepth, cfg.HistoryDepth)
	}
	if cfg.ExpiryDays != DefaultExpiryDays {
		t.Errorf("expected %d, got %d", DefaultExpiryDays, cfg.ExpiryDays)
	}
	if cfg.WarningDays != DefaultWarningDays {
		t.Errorf("expected %d, got %d", DefaultWarningDays, cfg.WarningDays)
	}
	if !cfg.Complexity.RequireUppercase {
		t.Error("expected RequireUppercase true")
	}
	if !cfg.Complexity.RequireLowercase {
		t.Error("expected RequireLowercase true")
	}
	if !cfg.Complexity.RequireDigit {
		t.Error("expected RequireDigit true")
	}
	if !cfg.Complexity.RequireSpecial {
		t.Error("expected RequireSpecial true")
	}
}

func TestValidatePassword(t *testing.T) {
	engine, _ := NewEngine()

	t.Run("valid password", func(t *testing.T) {
		vr := engine.ValidatePassword(validPassword)
		if !vr.Valid {
			t.Errorf("expected valid password, got violations: %v", vr.ErrorMessages())
		}
		if len(vr.Violations) != 0 {
			t.Errorf("expected 0 violations, got %d", len(vr.Violations))
		}
	})

	t.Run("too short", func(t *testing.T) {
		vr := engine.ValidatePassword("Ab1!")
		if vr.Valid {
			t.Error("expected invalid for too short")
		}
		found := false
		for _, v := range vr.Violations {
			if errors.Is(v.Err, ErrPasswordTooShort) {
				found = true
			}
		}
		if !found {
			t.Error("expected ErrPasswordTooShort violation")
		}
	})

	t.Run("missing uppercase", func(t *testing.T) {
		vr := engine.ValidatePassword("abcdefg1!")
		if vr.Valid {
			t.Error("expected invalid for missing uppercase")
		}
		found := false
		for _, v := range vr.Violations {
			if errors.Is(v.Err, ErrPasswordMissingUppercase) {
				found = true
			}
		}
		if !found {
			t.Error("expected ErrPasswordMissingUppercase violation")
		}
	})

	t.Run("missing lowercase", func(t *testing.T) {
		vr := engine.ValidatePassword("ABCDEFG1!")
		if vr.Valid {
			t.Error("expected invalid for missing lowercase")
		}
		found := false
		for _, v := range vr.Violations {
			if errors.Is(v.Err, ErrPasswordMissingLowercase) {
				found = true
			}
		}
		if !found {
			t.Error("expected ErrPasswordMissingLowercase violation")
		}
	})

	t.Run("missing digit", func(t *testing.T) {
		vr := engine.ValidatePassword("Abcdefgh!")
		if vr.Valid {
			t.Error("expected invalid for missing digit")
		}
		found := false
		for _, v := range vr.Violations {
			if errors.Is(v.Err, ErrPasswordMissingDigit) {
				found = true
			}
		}
		if !found {
			t.Error("expected ErrPasswordMissingDigit violation")
		}
	})

	t.Run("missing special", func(t *testing.T) {
		vr := engine.ValidatePassword("Abcdefg12")
		if vr.Valid {
			t.Error("expected invalid for missing special")
		}
		found := false
		for _, v := range vr.Violations {
			if errors.Is(v.Err, ErrPasswordMissingSpecial) {
				found = true
			}
		}
		if !found {
			t.Error("expected ErrPasswordMissingSpecial violation")
		}
	})

	t.Run("multiple violations", func(t *testing.T) {
		vr := engine.ValidatePassword("short")
		if vr.Valid {
			t.Error("expected invalid")
		}
		if len(vr.Violations) < 2 {
			t.Errorf("expected at least 2 violations, got %d", len(vr.Violations))
		}
		msgs := vr.ErrorMessages()
		if len(msgs) != len(vr.Violations) {
			t.Error("ErrorMessages count mismatch")
		}
		combined := vr.CombinedError()
		if combined == nil {
			t.Error("CombinedError should not be nil")
		}
	})

	t.Run("custom config relaxed complexity off", func(t *testing.T) {
		cfg := Config{
			MinLength:    6,
			BcryptCost: DefaultBcryptCost,
			HistoryDepth: 0,
			ExpiryDays: 0,
			WarningDays: 0,
			Complexity: ComplexityConfig{
				RequireUppercase: false,
				RequireLowercase: false,
				RequireDigit:     false,
				RequireSpecial:  false,
			},
		}
		engine2, _ := NewEngineWithConfig(cfg)
		vr := engine2.ValidatePassword("simple")
		if !vr.Valid {
			t.Errorf("expected valid, got violations: %v", vr.ErrorMessages())
		}
	})
}

func TestValidationResultCombinedError(t *testing.T) {
	vr := ValidationResult{Valid: true}
	if vr.CombinedError() != nil {
		t.Error("expected nil for valid result")
	}

	engine, _ := NewEngine()
	vr = engine.ValidatePassword("x")
	err := vr.CombinedError()
	if err == nil {
		t.Error("expected non-nil error")
	}
}

func TestSetPassword(t *testing.T) {
	engine, _ := NewEngine()

	t.Run("set password for new user", func(t *testing.T) {
		err := engine.SetPassword("user1", validPassword)
		if err != nil {
			t.Fatalf("SetPassword failed: %v", err)
		}
		if engine.UserCount() != 1 {
			t.Errorf("expected 1 user, got %d", engine.UserCount())
		}

		hash, cost, err := engine.GetPasswordHash("user1")
		if err != nil {
			t.Fatalf("GetPasswordHash failed: %v", err)
		}
		if cost != DefaultBcryptCost {
			t.Errorf("expected cost %d, got %d", DefaultBcryptCost, cost)
		}
		if len(hash) == 0 {
			t.Error("expected non-empty hash")
		}

		err = bcrypt.CompareHashAndPassword(hash, []byte(validPassword))
		if err != nil {
			t.Errorf("hash verification failed: %v", err)
		}
	})

	t.Run("update existing user password moves to history", func(t *testing.T) {
		err := engine.SetPassword("user1", "NewPass123!")
		if err != nil {
			t.Fatalf("SetPassword failed: %v", err)
		}

		history, err := engine.GetHistoryEntries("user1")
		if err != nil {
			t.Fatalf("GetHistoryEntries failed: %v", err)
		}
		if len(history) != 1 {
			t.Errorf("expected 1 history entry, got %d", len(history))
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		err := engine.SetPassword("", validPassword)
		if !errors.Is(err, ErrEmptyUserID) {
			t.Errorf("expected ErrEmptyUserID, got %v", err)
		}
	})

	t.Run("empty password", func(t *testing.T) {
		err := engine.SetPassword("userx", "")
		if !errors.Is(err, ErrEmptyPassword) {
			t.Errorf("expected ErrEmptyPassword, got %v", err)
		}
	})

	t.Run("invalid password", func(t *testing.T) {
		err := engine.SetPassword("userx", "weak")
		if err == nil {
			t.Error("expected error for weak password")
		}
	})
}

func TestChangePassword(t *testing.T) {
	engine, _ := NewEngine()
	engine.SetPassword("user1", validPassword)

	t.Run("successful change", func(t *testing.T) {
		err := engine.ChangePassword("user1", validPassword, "NewPass456!")
		if err != nil {
			t.Fatalf("ChangePassword failed: %v", err)
		}
		history, err := engine.GetHistoryEntries("user1")
		if err != nil {
			t.Fatalf("GetHistoryEntries failed: %v", err)
		}
		if len(history) != 1 {
			t.Errorf("expected 1 history entry, got %d", len(history))
		}
	})

	t.Run("wrong old password", func(t *testing.T) {
		err := engine.ChangePassword("user1", "WrongPass1!", "Another789!")
		if !errors.Is(err, ErrPasswordMismatch) {
			t.Errorf("expected ErrPasswordMismatch, got %v", err)
		}
	})

	t.Run("reuse current password", func(t *testing.T) {
		engine.SetPassword("user2", validPassword)
		err := engine.ChangePassword("user2", validPassword, validPassword)
		if !errors.Is(err, ErrPasswordHistoryReused) {
			t.Errorf("expected ErrPasswordHistoryReused, got %v", err)
		}
	})

	t.Run("reuse history password", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 6
		engine3, _ := NewEngineWithConfig(cfg)
		engine3.SetPassword("u", "Pass1!")
		time.Sleep(10 * time.Millisecond)
		engine3.SetPassword("u", "Pass2!")
		time.Sleep(10 * time.Millisecond)
		engine3.SetPassword("u", "Pass3!")

		err := engine3.ChangePassword("u", "Pass3!", "Pass1!")
		if !errors.Is(err, ErrPasswordHistoryReused) {
			t.Errorf("expected ErrPasswordHistoryReused, got %v", err)
		}
	})

	t.Run("history depth exceeded ok", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 6
		cfg.HistoryDepth = 2
		engine4, _ := NewEngineWithConfig(cfg)
		engine4.SetPassword("u", "Pass1!")
		engine4.SetPassword("u", "Pass2!")
		engine4.SetPassword("u", "Pass3!")
		err := engine4.ChangePassword("u", "Pass3!", "Pass1!")
		if !errors.Is(err, ErrPasswordHistoryReused) {
			t.Errorf("expected ErrPasswordHistoryReused for password in history depth, got %v", err)
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		err := engine.ChangePassword("", "a", "b")
		if !errors.Is(err, ErrEmptyUserID) {
			t.Errorf("expected ErrEmptyUserID, got %v", err)
		}
	})

	t.Run("empty new password", func(t *testing.T) {
		err := engine.ChangePassword("user1", validPassword, "")
		if !errors.Is(err, ErrEmptyPassword) {
			t.Errorf("expected ErrEmptyPassword, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		err := engine.ChangePassword("nonexistent", validPassword, "NewPass123!")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("invalid new password", func(t *testing.T) {
		err := engine.ChangePassword("user1", "NewPass456!", "weak")
		if err == nil {
			t.Error("expected error for weak new password")
		}
	})
}

func TestVerifyPassword(t *testing.T) {
	engine, _ := NewEngine()
	engine.SetPassword("user1", validPassword)

	t.Run("correct password", func(t *testing.T) {
		result, err := engine.VerifyPassword("user1", validPassword)
		if err != nil {
			t.Fatalf("VerifyPassword failed: %v", err)
		}
		if !result.Valid {
			t.Error("expected Valid true")
		}
		if result.Rehashed {
			t.Error("expected Rehashed false")
		}
		if result.PasswordState == nil {
			t.Fatal("expected PasswordState not nil")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		result, err := engine.VerifyPassword("user1", "WrongPass1!")
		if !errors.Is(err, ErrPasswordMismatch) {
			t.Errorf("expected ErrPasswordMismatch, got %v", err)
		}
		if result.Valid {
			t.Error("expected Valid false")
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		_, err := engine.VerifyPassword("", validPassword)
		if !errors.Is(err, ErrEmptyUserID) {
			t.Errorf("expected ErrEmptyUserID, got %v", err)
		}
	})

	t.Run("empty password", func(t *testing.T) {
		_, err := engine.VerifyPassword("user1", "")
		if !errors.Is(err, ErrEmptyPassword) {
			t.Errorf("expected ErrEmptyPassword, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		_, err := engine.VerifyPassword("nonexistent", validPassword)
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("expired password", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ExpiryDays = 1
		engine2, _ := NewEngineWithConfig(cfg)
		past := time.Now().Add(-48 * time.Hour)
		engine2.SetNowFunc(func() time.Time { return past })
		engine2.SetPassword("expired_user", validPassword)
		engine2.SetNowFunc(time.Now)

		_, err := engine2.VerifyPassword("expired_user", validPassword)
		if !errors.Is(err, ErrPasswordExpired) {
			t.Errorf("expected ErrPasswordExpired, got %v", err)
		}
	})

	t.Run("auto rehash on higher cost", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.BcryptCost = 4
		engine3, _ := NewEngineWithConfig(cfg)
		engine3.SetPassword("rehash_user", validPassword)
		_, costBefore, _ := engine3.GetPasswordHash("rehash_user")
		if costBefore != 4 {
			t.Fatalf("expected initial cost 4, got %d", costBefore)
		}

		err := engine3.UpdateBcryptCost(10)
		if err != nil {
			t.Fatalf("UpdateBcryptCost failed: %v", err)
		}

		result, err := engine3.VerifyPassword("rehash_user", validPassword)
		if err != nil {
			t.Fatalf("VerifyPassword after cost upgrade failed: %v", err)
		}
		if !result.Valid {
			t.Error("expected valid")
		}
		if !result.Rehashed {
			t.Error("expected Rehashed true")
		}
		if result.NewCost != 10 {
			t.Errorf("expected NewCost 10, got %d", result.NewCost)
		}

		_, costAfter, _ := engine3.GetPasswordHash("rehash_user")
		if costAfter != 10 {
			t.Errorf("expected stored cost 10, got %d", costAfter)
		}
	})

	t.Run("must change password", func(t *testing.T) {
		engine4, _ := NewEngine()
		engine4.SetPassword("mustchange", validPassword)
		engine4.ForcePasswordChange("mustchange")
		_, err := engine4.VerifyPassword("mustchange", validPassword)
		if !errors.Is(err, ErrMustChangePassword) {
			t.Errorf("expected ErrMustChangePassword for must change, got %v", err)
		}
		if errors.Is(err, ErrPasswordExpired) {
			t.Error("should not return ErrPasswordExpired for must change")
		}
	})
}

func TestGetPasswordStatus(t *testing.T) {
	engine, _ := NewEngine()
	engine.SetPassword("user1", validPassword)

	t.Run("normal status", func(t *testing.T) {
		status, err := engine.GetPasswordStatus("user1")
		if err != nil {
			t.Fatalf("GetPasswordStatus failed: %v", err)
		}
		if status.UserID != "user1" {
			t.Errorf("expected UserID user1, got %s", status.UserID)
		}
		if status.IsExpired {
			t.Error("expected not expired")
		}
		if status.DaysRemaining != DefaultExpiryDays {
			t.Errorf("expected DaysRemaining %d, got %d", DefaultExpiryDays, status.DaysRemaining)
		}
		if status.DaysSinceChanged != 0 {
			t.Errorf("expected DaysSinceChanged 0, got %d", status.DaysSinceChanged)
		}
		if status.IsWarningPeriod {
			t.Error("expected not in warning period")
		}
		if status.MustChange {
			t.Error("expected MustChange false")
		}
	})

	t.Run("warning period", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ExpiryDays = 10
		cfg.WarningDays = 7
		engine2, _ := NewEngineWithConfig(cfg)
		past := time.Now().Add(-8 * 24 * time.Hour)
		engine2.SetNowFunc(func() time.Time { return past })
		engine2.SetPassword("warn_user", validPassword)
		engine2.SetNowFunc(time.Now)

		status, err := engine2.GetPasswordStatus("warn_user")
		if err != nil {
			t.Fatalf("GetPasswordStatus failed: %v", err)
		}
		if !status.IsWarningPeriod {
			t.Error("expected in warning period")
		}
		if status.DaysRemaining != 2 {
			t.Errorf("expected DaysRemaining 2, got %d", status.DaysRemaining)
		}
	})

	t.Run("expired status", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ExpiryDays = 1
		engine3, _ := NewEngineWithConfig(cfg)
		past := time.Now().Add(-48 * time.Hour)
		engine3.SetNowFunc(func() time.Time { return past })
		engine3.SetPassword("exp_user", validPassword)
		engine3.SetNowFunc(time.Now)

		status, err := engine3.GetPasswordStatus("exp_user")
		if err != nil {
			t.Fatalf("GetPasswordStatus failed: %v", err)
		}
		if !status.IsExpired {
			t.Error("expected expired")
		}
		if status.DaysRemaining > 0 {
			t.Errorf("expected DaysRemaining <= 0, got %d", status.DaysRemaining)
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		_, err := engine.GetPasswordStatus("")
		if !errors.Is(err, ErrEmptyUserID) {
			t.Errorf("expected ErrEmptyUserID, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		_, err := engine.GetPasswordStatus("nonexistent")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestForcePasswordChange(t *testing.T) {
	engine, _ := NewEngine()
	engine.SetPassword("user1", validPassword)

	t.Run("force change", func(t *testing.T) {
		err := engine.ForcePasswordChange("user1")
		if err != nil {
			t.Fatalf("ForcePasswordChange failed: %v", err)
		}
		status, _ := engine.GetPasswordStatus("user1")
		if !status.MustChange {
			t.Error("expected MustChange true")
		}
	})

	t.Run("after reset after set password", func(t *testing.T) {
		engine.SetPassword("user1", "NewPass123!")
		status, _ := engine.GetPasswordStatus("user1")
		if status.MustChange {
			t.Error("expected MustChange false after SetPassword")
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		err := engine.ForcePasswordChange("")
		if !errors.Is(err, ErrEmptyUserID) {
			t.Errorf("expected ErrEmptyUserID, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		err := engine.ForcePasswordChange("nonexistent")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestDeleteUser(t *testing.T) {
	engine, _ := NewEngine()
	engine.SetPassword("user1", validPassword)

	t.Run("delete existing user", func(t *testing.T) {
		err := engine.DeleteUser("user1")
		if err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}
		if engine.UserCount() != 0 {
			t.Errorf("expected 0 users, got %d", engine.UserCount())
		}
	})

	t.Run("empty user id", func(t *testing.T) {
		err := engine.DeleteUser("")
		if !errors.Is(err, ErrEmptyUserID) {
			t.Errorf("expected ErrEmptyUserID, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		err := engine.DeleteUser("nonexistent")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestUpdateBcryptCost(t *testing.T) {
	engine, _ := NewEngine()

	t.Run("valid cost", func(t *testing.T) {
		err := engine.UpdateBcryptCost(12)
		if err != nil {
			t.Fatalf("UpdateBcryptCost failed: %v", err)
		}
		cfg := engine.GetConfig()
		if cfg.BcryptCost != 12 {
			t.Errorf("expected BcryptCost 12, got %d", cfg.BcryptCost)
		}
	})

	t.Run("invalid cost too low", func(t *testing.T) {
		err := engine.UpdateBcryptCost(MinBcryptCost - 1)
		if !errors.Is(err, ErrInvalidBcryptCost) {
			t.Errorf("expected ErrInvalidBcryptCost, got %v", err)
		}
	})

	t.Run("invalid cost too high", func(t *testing.T) {
		err := engine.UpdateBcryptCost(MaxBcryptCost + 1)
		if !errors.Is(err, ErrInvalidBcryptCost) {
			t.Errorf("expected ErrInvalidBcryptCost, got %v", err)
		}
	})
}

func TestGetPasswordHash(t *testing.T) {
	engine, _ := NewEngine()

	t.Run("empty user id", func(t *testing.T) {
		_, _, err := engine.GetPasswordHash("")
		if !errors.Is(err, ErrEmptyUserID) {
			t.Errorf("expected ErrEmptyUserID, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		_, _, err := engine.GetPasswordHash("nonexistent")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestGetHistoryEntries(t *testing.T) {
	engine, _ := NewEngine()

	t.Run("empty user id", func(t *testing.T) {
		_, err := engine.GetHistoryEntries("")
		if !errors.Is(err, ErrEmptyUserID) {
			t.Errorf("expected ErrEmptyUserID, got %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		_, err := engine.GetHistoryEntries("nonexistent")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("no history", func(t *testing.T) {
		engine.SetPassword("newhist", validPassword)
		h, err := engine.GetHistoryEntries("newhist")
		if err != nil {
			t.Fatalf("GetHistoryEntries failed: %v", err)
		}
		if len(h) != 0 {
			t.Errorf("expected 0 history, got %d", len(h))
		}
	})
}

func TestHistoryDepthPruning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinLength = 6
	cfg.HistoryDepth = 3
	engine, _ := NewEngineWithConfig(cfg)

	passwords := []string{
		"Pass1!",
		"Pass2!",
		"Pass3!",
		"Pass4!",
		"Pass5!",
	}

	for i := 0; i < len(passwords); i++ {
		engine.SetPassword("prune_user", passwords[i])
	}

	history, err := engine.GetHistoryEntries("prune_user")
	if err != nil {
		t.Fatalf("GetHistoryEntries failed: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("expected history depth 3, got %d", len(history))
	}
}

func TestConcurrentOperations(t *testing.T) {
	engine, _ := NewEngine()

	var wg sync.WaitGroup
	errorsCount := 0
	var mu sync.Mutex

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			userID := fmt.Sprintf("concurrent_user_%d", i%10)
			pw := fmt.Sprintf("ConcurrentPass%d!", i%100)
			err := engine.SetPassword(userID, pw)
			if err != nil {
				mu.Lock()
				errorsCount++
				mu.Unlock()
			}
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			userID := fmt.Sprintf("concurrent_user_%d", i%10)
			pw := fmt.Sprintf("ConcurrentPass%d!", i%100)
			engine.VerifyPassword(userID, pw)
		}(i)
	}

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			userID := fmt.Sprintf("concurrent_user_%d", i%10)
			engine.GetPasswordStatus(userID)
			engine.GetHistoryEntries(userID)
			engine.UserCount()
		}(i)
	}

	wg.Wait()

	if errorsCount > 0 {
		t.Errorf("expected 0 errors in concurrent SetPassword, got %d", errorsCount)
	}
}

func TestSetNowFunc(t *testing.T) {
	engine, _ := NewEngine()
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	engine.SetNowFunc(func() time.Time {
		return fixedTime
	})

	engine.SetPassword("timetest", validPassword)
	status, _ := engine.GetPasswordStatus("timetest")
	if !status.CreatedAt.Equal(fixedTime) {
		t.Errorf("expected CreatedAt %v, got %v", fixedTime, status.CreatedAt)
	}
}

func TestComplexityEdgeCases(t *testing.T) {
	cfg := DefaultConfig()
	engine, _ := NewEngineWithConfig(cfg)

	testCases := []struct {
		name     string
		password string
		valid    bool
	}{
		{"all requirements met", "Abcdefg1!", true},
		{"exact min length", "Abcdef1!", true},
		{"unicode uppercase", "Äbcdefg1!", true},
		{"multiple specials", "Abcdef1!!", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vr := engine.ValidatePassword(tc.password)
			if vr.Valid != tc.valid {
				t.Errorf("password %q: expected valid=%v, got valid=%v, violations=%v",
					tc.password, tc.valid, vr.Valid, vr.ErrorMessages())
			}
		})
	}
}

func TestSpecialChars(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinLength = 6
	engine, _ := NewEngineWithConfig(cfg)

	specials := []string{"!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "-", "_", "=", "+", "[", "]", "{", "}", ";", ":", ",", ".", "<", ">", "/", "?", "~", "`", "|", "\\"}

	for _, sp := range specials {
		pw := "Ab1" + sp + "xyz"
		vr := engine.ValidatePassword(pw)
		if vr.Valid {
			t.Logf("special char %q works", sp)
		} else {
			t.Errorf("special char %q should be accepted, password=%q, errors: %v", sp, pw, vr.ErrorMessages())
		}
	}
}

func TestZeroExpiryDays(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ExpiryDays = 0
	engine, _ := NewEngineWithConfig(cfg)
	engine.SetPassword("noexpiry", validPassword)

	status, err := engine.GetPasswordStatus("noexpiry")
	if err != nil {
		t.Fatalf("GetPasswordStatus failed: %v", err)
	}
	if status.IsExpired {
		t.Error("expected not expired with ExpiryDays=0")
	}
	if status.IsWarningPeriod {
		t.Error("expected no warning period with ExpiryDays=0")
	}

	result, err := engine.VerifyPassword("noexpiry", validPassword)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid with ExpiryDays=0")
	}
}

func TestFullLifecycle(t *testing.T) {
	engine, _ := NewEngine()

	err := engine.SetPassword("lifecycle_user", validPassword)
	if err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}

	status, err := engine.GetPasswordStatus("lifecycle_user")
	if err != nil {
		t.Fatalf("GetPasswordStatus failed: %v", err)
	}
	if status.IsExpired {
		t.Error("expected not expired initially")
	}

	result, err := engine.VerifyPassword("lifecycle_user", validPassword)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid password")
	}

	err = engine.ChangePassword("lifecycle_user", validPassword, "NewPass789!")
	if err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	history, err := engine.GetHistoryEntries("lifecycle_user")
	if err != nil {
		t.Fatalf("GetHistoryEntries failed: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}

	err = engine.ForcePasswordChange("lifecycle_user")
	if err != nil {
		t.Fatalf("ForcePasswordChange failed: %v", err)
	}

	_, err = engine.VerifyPassword("lifecycle_user", "NewPass789!")
	if !errors.Is(err, ErrMustChangePassword) {
		t.Errorf("expected ErrMustChangePassword after force change, got %v", err)
	}

	err = engine.SetPassword("lifecycle_user", "FinalPass000!")
	if err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}

	result, err = engine.VerifyPassword("lifecycle_user", "FinalPass000!")
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid after reset")
	}

	err = engine.DeleteUser("lifecycle_user")
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	if engine.UserCount() != 0 {
		t.Errorf("expected 0 users after delete, got %d", engine.UserCount())
	}
}

func TestErrorStrings(t *testing.T) {
	errStrs := map[error]string{
		ErrEmptyUserID:               "user id cannot be empty",
		ErrEmptyPassword:             "password cannot be empty",
		ErrUserNotFound:              "user not found",
		ErrPasswordExpired:           "password has expired",
		ErrMustChangePassword:        "password must be changed",
		ErrPasswordHistoryReused:     "cannot reuse recent passwords",
		ErrPasswordMismatch:          "password mismatch",
		ErrPasswordTooShort:          "password too short",
		ErrPasswordMissingUppercase:  "must contain uppercase",
		ErrPasswordMissingLowercase:  "must contain lowercase",
		ErrPasswordMissingDigit:      "must contain digit",
		ErrPasswordMissingSpecial:    "must contain special",
	}

	for err, substr := range errStrs {
		if !strings.Contains(err.Error(), substr) {
			t.Errorf("error %q should contain %q", err.Error(), substr)
		}
	}
}

func TestHistoryDepthZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinLength = 6
	cfg.HistoryDepth = 0
	engine, _ := NewEngineWithConfig(cfg)

	engine.SetPassword("user", "Pass1!")
	engine.SetPassword("user", "Pass2!")
	engine.SetPassword("user", "Pass3!")

	history, err := engine.GetHistoryEntries("user")
	if err != nil {
		t.Fatalf("GetHistoryEntries failed: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected 0 history with depth 0, got %d", len(history))
	}

	err = engine.ChangePassword("user", "Pass3!", "Pass1!")
	if err != nil {
		t.Errorf("expected no history check with depth 0, got: %v", err)
	}
}

func TestSetPassword_HistoryCheck(t *testing.T) {
	t.Run("admin reset reuses recent password", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 8
		cfg.HistoryDepth = 3
		engine, err := NewEngineWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewEngineWithConfig failed: %v", err)
		}

		pass1 := "Password1!"
		pass2 := "Password2@"
		pass3 := "Password3#"

		err = engine.SetPassword("admin_user", pass1)
		if err != nil {
			t.Fatalf("SetPassword pass1 failed: %v", err)
		}

		err = engine.SetPassword("admin_user", pass2)
		if err != nil {
			t.Fatalf("SetPassword pass2 failed: %v", err)
		}

		err = engine.SetPassword("admin_user", pass3)
		if err != nil {
			t.Fatalf("SetPassword pass3 failed: %v", err)
		}

		err = engine.SetPassword("admin_user", pass1)
		if !errors.Is(err, ErrPasswordHistoryReused) {
			t.Errorf("expected ErrPasswordHistoryReused when reusing pass1, got %v", err)
		}

		err = engine.SetPassword("admin_user", pass2)
		if !errors.Is(err, ErrPasswordHistoryReused) {
			t.Errorf("expected ErrPasswordHistoryReused when reusing pass2, got %v", err)
		}
	})

	t.Run("new user has no history check", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 8
		cfg.HistoryDepth = 3
		engine, _ := NewEngineWithConfig(cfg)

		err := engine.SetPassword("new_user", "FreshPass1!")
		if err != nil {
			t.Errorf("expected no error for new user, got %v", err)
		}
	})

	t.Run("history depth zero skips check", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 8
		cfg.HistoryDepth = 0
		engine, _ := NewEngineWithConfig(cfg)

		err := engine.SetPassword("user_no_history", "SamePass1!")
		if err != nil {
			t.Fatalf("First SetPassword failed: %v", err)
		}

		err = engine.SetPassword("user_no_history", "SamePass1!")
		if err != nil {
			t.Errorf("expected no history check with depth 0, got %v", err)
		}
	})
}

func TestValidatePassword_ConcurrentConfigUpdate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinLength = 8
	engine, _ := NewEngineWithConfig(cfg)

	done := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			engine.ValidatePassword("TestPass1!")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			newCost := MinBcryptCost + (i % (MaxBcryptCost - MinBcryptCost + 1))
			_ = engine.UpdateBcryptCost(newCost)
			time.Sleep(10 * time.Microsecond)
		}
		close(done)
	}()

	go func() {
		<-done
	}()

	wg.Wait()
}

func TestVerifyPassword_MustChangeVsExpired(t *testing.T) {
	t.Run("must change returns ErrMustChangePassword", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 8
		cfg.ExpiryDays = 90
		engine, _ := NewEngineWithConfig(cfg)

		err := engine.SetPassword("must_change_user", "TestPass1!")
		if err != nil {
			t.Fatalf("SetPassword failed: %v", err)
		}

		err = engine.ForcePasswordChange("must_change_user")
		if err != nil {
			t.Fatalf("ForcePasswordChange failed: %v", err)
		}

		_, err = engine.VerifyPassword("must_change_user", "TestPass1!")
		if !errors.Is(err, ErrMustChangePassword) {
			t.Errorf("expected ErrMustChangePassword, got %v", err)
		}
		if errors.Is(err, ErrPasswordExpired) {
			t.Error("should not return ErrPasswordExpired for must change")
		}
	})

	t.Run("naturally expired returns ErrPasswordExpired", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 8
		cfg.ExpiryDays = 1
		engine, _ := NewEngineWithConfig(cfg)

		err := engine.SetPassword("expired_user", "TestPass1!")
		if err != nil {
			t.Fatalf("SetPassword failed: %v", err)
		}

		engine.SetNowFunc(func() time.Time {
			return time.Now().Add(48 * time.Hour)
		})

		_, err = engine.VerifyPassword("expired_user", "TestPass1!")
		if !errors.Is(err, ErrPasswordExpired) {
			t.Errorf("expected ErrPasswordExpired, got %v", err)
		}
		if errors.Is(err, ErrMustChangePassword) {
			t.Error("should not return ErrMustChangePassword for natural expiry")
		}
	})
}

func TestVerifyPassword_ConcurrentCostUpdate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinLength = 8
	cfg.BcryptCost = MinBcryptCost
	engine, _ := NewEngineWithConfig(cfg)

	err := engine.SetPassword("rehash_user", "TestPass1!")
	if err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}

	var wg sync.WaitGroup
	verifyCount := 500
	updateCount := 50

	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < verifyCount; i++ {
			result, err := engine.VerifyPassword("rehash_user", "TestPass1!")
			if err != nil && !errors.Is(err, ErrPasswordExpired) && !errors.Is(err, ErrMustChangePassword) {
				t.Errorf("VerifyPassword error: %v", err)
			}
			_ = result
			time.Sleep(1 * time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < updateCount; i++ {
			newCost := MinBcryptCost + (i % 3)
			if newCost > MaxBcryptCost {
				newCost = MaxBcryptCost
			}
			_ = engine.UpdateBcryptCost(newCost)
			time.Sleep(10 * time.Microsecond)
		}
	}()

	wg.Wait()

	_, currentCost, err := engine.GetPasswordHash("rehash_user")
	if err != nil {
		t.Fatalf("GetPasswordHash failed: %v", err)
	}
	if currentCost < MinBcryptCost || currentCost > MaxBcryptCost {
		t.Errorf("cost out of range after concurrent updates: %d", currentCost)
	}
}

func TestChangePassword_TimeOfCheckToTimeOfUse(t *testing.T) {
	t.Run("concurrent change password race detection", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 8
		cfg.HistoryDepth = 5
		engine, _ := NewEngineWithConfig(cfg)

		pass0 := "OriginalPass1!"
		pass1 := "PasswordOne1!"

		err := engine.SetPassword("race_user", pass0)
		if err != nil {
			t.Fatalf("SetPassword failed: %v", err)
		}

		var wg sync.WaitGroup
		errs := make([]error, 2)

		wg.Add(2)

		go func() {
			defer wg.Done()
			errs[0] = engine.ChangePassword("race_user", pass0, pass1)
		}()

		go func() {
			defer wg.Done()
			errs[1] = engine.ChangePassword("race_user", pass0, pass1)
		}()

		wg.Wait()

		successCount := 0
		failureCount := 0
		for _, e := range errs {
			if e == nil {
				successCount++
			} else if errors.Is(e, ErrPasswordHistoryReused) || errors.Is(e, ErrPasswordMismatch) {
				failureCount++
			}
		}

		if successCount != 1 {
			t.Errorf("expected exactly 1 success, got %d successes, errors: %v", successCount, errs)
		}

		history, err := engine.GetHistoryEntries("race_user")
		if err != nil {
			t.Fatalf("GetHistoryEntries failed: %v", err)
		}
		if len(history) != 1 {
			t.Errorf("expected 1 history entry, got %d", len(history))
		}
	})

	t.Run("reuse password set by concurrent goroutine", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinLength = 8
		cfg.HistoryDepth = 5
		engine, _ := NewEngineWithConfig(cfg)

		initial := "StartPass0!"
		passA := "PasswordAAA1!"
		passB := "PasswordBBB2@"

		err := engine.SetPassword("concurrent_user", initial)
		if err != nil {
			t.Fatalf("SetPassword failed: %v", err)
		}

		barrier := make(chan struct{})
		var wg sync.WaitGroup
		var mu sync.Mutex
		results := make([]error, 2)

		wg.Add(2)

		go func() {
			defer wg.Done()
			<-barrier
			err := engine.ChangePassword("concurrent_user", initial, passA)
			mu.Lock()
			results[0] = err
			mu.Unlock()
		}()

		go func() {
			defer wg.Done()
			<-barrier
			err := engine.ChangePassword("concurrent_user", initial, passB)
			mu.Lock()
			results[1] = err
			mu.Unlock()
		}()

		close(barrier)
		wg.Wait()

		err = engine.SetPassword("concurrent_user", passA)
		if !errors.Is(err, ErrPasswordHistoryReused) {
			t.Errorf("SetPassword should detect passA in history, got %v", err)
		}

		err = engine.SetPassword("concurrent_user", passB)
		if !errors.Is(err, ErrPasswordHistoryReused) {
			t.Errorf("SetPassword should detect passB in history, got %v", err)
		}
	})
}

func TestErrors_IsCheck(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrMustChangePassword", ErrMustChangePassword},
		{"ErrPasswordExpired", ErrPasswordExpired},
		{"ErrPasswordHistoryReused", ErrPasswordHistoryReused},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("errors.Is should match same error")
			}
		})
	}

	if errors.Is(ErrMustChangePassword, ErrPasswordExpired) {
		t.Error("ErrMustChangePassword should not match ErrPasswordExpired")
	}
	if errors.Is(ErrPasswordExpired, ErrMustChangePassword) {
		t.Error("ErrPasswordExpired should not match ErrMustChangePassword")
	}
}
