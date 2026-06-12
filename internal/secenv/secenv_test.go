package secenv

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"sync"
	"testing"
	"time"
)

func TestNewSecureEnvelope(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}
	if se == nil {
		t.Fatal("NewSecureEnvelope returned nil")
	}
	if se.keyManager == nil {
		t.Fatal("keyManager is nil")
	}
	if se.replayGuard == nil {
		t.Fatal("replayGuard is nil")
	}
	if se.keyManager.CurrentVersion() != 1 {
		t.Errorf("expected version 1, got %d", se.keyManager.CurrentVersion())
	}
}

func TestNewSecureEnvelopeWithConfig(t *testing.T) {
	cfg := &Config{
		MaxKeys:      5,
		ReplayWindow: 500,
	}
	se, err := NewSecureEnvelope(cfg)
	if err != nil {
		t.Fatalf("NewSecureEnvelope with config failed: %v", err)
	}
	if se.keyManager.MaxKeys() != 5 {
		t.Errorf("expected max keys 5, got %d", se.keyManager.MaxKeys())
	}
}

func TestEncryptDecryptBasic(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("Hello, Secure Envelope!")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if len(envelope) == 0 {
		t.Fatal("Encrypt returned empty envelope")
	}

	decrypted, err := se.Decrypt(envelope)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("decrypted data mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptEmptyData(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	_, err = se.Encrypt([]byte{})
	if err != ErrEmptyData {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}

	_, err = se.Encrypt(nil)
	if err != ErrEmptyData {
		t.Errorf("expected ErrEmptyData for nil, got %v", err)
	}
}

func TestDecryptEmptyData(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	_, err = se.Decrypt([]byte{})
	if err != ErrEmptyData {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}

	_, err = se.Decrypt(nil)
	if err != ErrEmptyData {
		t.Errorf("expected ErrEmptyData for nil, got %v", err)
	}
}

func TestDecryptInvalidFormat(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	shortData := make([]byte, 10)
	_, err = se.Decrypt(shortData)
	if err != ErrInvalidFormat {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecryptInvalidVersion(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("test")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	envelope[0] = 99

	_, err = se.Decrypt(envelope)
	if err != ErrInvalidVersion {
		t.Errorf("expected ErrInvalidVersion, got %v", err)
	}
}

func TestDecryptKeyNotFound(t *testing.T) {
	se1, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope se1 failed: %v", err)
	}

	se2, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope se2 failed: %v", err)
	}

	kv, err := NewKeyVersion(999)
	if err != nil {
		t.Fatalf("NewKeyVersion failed: %v", err)
	}
	if err := se1.AddKey(kv); err != nil {
		t.Fatalf("AddKey failed: %v", err)
	}

	plaintext := []byte("test")
	envelope, err := se1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = se2.Decrypt(envelope)
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestHMACSignatureTampering(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("sensitive data")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	envelope[len(envelope)-1] ^= 0xFF

	_, err = se.Decrypt(envelope)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestCiphertextTampering(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("sensitive data")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	offset := 1 + 4 + 8 + NonceSize
	envelope[offset] ^= 0xFF

	_, err = se.Decrypt(envelope)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature (HMAC catches ciphertext tampering first), got %v", err)
	}
}

func TestGCMTagTampering(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("sensitive data")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	gcmTagOffset := len(envelope) - GCMTagSize - HMACSize
	envelope[gcmTagOffset] ^= 0xFF

	_, err = se.Decrypt(envelope)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature (HMAC covers GCM tag), got %v", err)
	}
}

func TestGCMTagValidationWithValidHMAC(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("test data")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	env, err := se.deserialize(envelope)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	env.GCMTag[0] ^= 0xFF

	kv, _ := se.keyManager.GetKey(env.KeyVersion)
	newSig, err := se.computeHMAC(env, kv.SignKey)
	if err != nil {
		t.Fatalf("computeHMAC failed: %v", err)
	}
	env.HMACSignature = newSig

	tamperedEnvelope, err := se.serialize(env)
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	se2 := NewSecureEnvelopeWithKeyManager(se.GetKeyManager(), 1000)
	_, err = se2.Decrypt(tamperedEnvelope)
	if err != ErrInvalidTag {
		t.Errorf("expected ErrInvalidTag for tampered GCM tag with valid HMAC, got %v", err)
	}
}

func TestReplayAttackDetection(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("test message")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = se.Decrypt(envelope)
	if err != nil {
		t.Fatalf("First decrypt failed: %v", err)
	}

	_, err = se.Decrypt(envelope)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected, got %v", err)
	}
}

func TestReplayWindow(t *testing.T) {
	rp := NewReplayProtector(100)

	err := rp.CheckAndUpdate(1, 1)
	if err != nil {
		t.Fatalf("CheckAndUpdate(1) failed: %v", err)
	}
	if rp.GetMaxSequence(1) != 1 {
		t.Errorf("expected max sequence 1, got %d", rp.GetMaxSequence(1))
	}

	err = rp.CheckAndUpdate(1, 101)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected for seq >= max+window (101 >= 1+100), got %v", err)
	}

	err = rp.CheckAndUpdate(1, 100)
	if err != nil {
		t.Fatalf("CheckAndUpdate(100) should succeed (100 < 1+100), got %v", err)
	}
	if rp.GetMaxSequence(1) != 100 {
		t.Errorf("expected max sequence 100, got %d", rp.GetMaxSequence(1))
	}

	err = rp.CheckAndUpdate(1, 50)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected for seq <= max (50 <= 100), got %v", err)
	}

	err = rp.CheckAndUpdate(1, 99)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected for seq <= max (99 <= 100), got %v", err)
	}

	err = rp.CheckAndUpdate(1, 200)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected for seq >= max+window (200 >= 100+100), got %v", err)
	}

	err = rp.CheckAndUpdate(1, 101)
	if err != nil {
		t.Fatalf("CheckAndUpdate(101) should succeed (101 > 100 and 101 < 100+100), got %v", err)
	}
	if rp.GetMaxSequence(1) != 101 {
		t.Errorf("expected max sequence 101, got %d", rp.GetMaxSequence(1))
	}

	err = rp.CheckAndUpdate(1, 201)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected for seq >= max+window (201 >= 101+100), got %v", err)
	}

	err = rp.CheckAndUpdate(1, 200)
	if err != nil {
		t.Fatalf("CheckAndUpdate(200) should succeed (200 < 101+100), got %v", err)
	}
	if rp.GetMaxSequence(1) != 200 {
		t.Errorf("expected max sequence 200, got %d", rp.GetMaxSequence(1))
	}

	err = rp.CheckAndUpdate(1, 300)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected for seq >= max+window (300 >= 200+100), got %v", err)
	}

	err = rp.CheckAndUpdate(1, 150)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected for seq <= max (150 <= 200), got %v", err)
	}

	err = rp.CheckAndUpdate(1, 299)
	if err != nil {
		t.Fatalf("CheckAndUpdate(299) should succeed (299 < 200+100), got %v", err)
	}
	if rp.GetMaxSequence(1) != 299 {
		t.Errorf("expected max sequence 299, got %d", rp.GetMaxSequence(1))
	}
}

func TestReplayProtectorReset(t *testing.T) {
	rp := NewReplayProtector(100)

	err := rp.CheckAndUpdate(1, 5)
	if err != nil {
		t.Fatalf("CheckAndUpdate failed: %v", err)
	}

	rp.Reset()

	err = rp.CheckAndUpdate(1, 3)
	if err != nil {
		t.Errorf("After reset, CheckAndUpdate(3) should succeed, got %v", err)
	}
}

func TestReplayMultipleKeyVersions(t *testing.T) {
	rp := NewReplayProtector(100)

	err := rp.CheckAndUpdate(1, 5)
	if err != nil {
		t.Fatalf("CheckAndUpdate for v1 failed: %v", err)
	}

	err = rp.CheckAndUpdate(2, 3)
	if err != nil {
		t.Errorf("CheckAndUpdate for v2 should not be affected by v1, got %v", err)
	}

	err = rp.CheckAndUpdate(1, 3)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected for v1 seq 3, got %v", err)
	}
}

func TestKeyRotation(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	initialVersion := se.keyManager.CurrentVersion()
	if initialVersion != 1 {
		t.Errorf("expected initial version 1, got %d", initialVersion)
	}

	kv, err := se.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}
	if kv == nil {
		t.Fatal("RotateKey returned nil")
	}
	if kv.Version != 2 {
		t.Errorf("expected rotated version 2, got %d", kv.Version)
	}
	if se.keyManager.CurrentVersion() != 2 {
		t.Errorf("expected current version 2 after rotation, got %d", se.keyManager.CurrentVersion())
	}

	kv2, err := se.RotateKey()
	if err != nil {
		t.Fatalf("Second RotateKey failed: %v", err)
	}
	if kv2.Version != 3 {
		t.Errorf("expected rotated version 3, got %d", kv2.Version)
	}
}

func TestDecryptWithOldKeyVersion(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext1 := []byte("message with key v1")
	envelope1, err := se.Encrypt(plaintext1)
	if err != nil {
		t.Fatalf("Encrypt v1 failed: %v", err)
	}

	_, err = se.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}

	plaintext2 := []byte("message with key v2")
	envelope2, err := se.Encrypt(plaintext2)
	if err != nil {
		t.Fatalf("Encrypt v2 failed: %v", err)
	}

	se2 := NewSecureEnvelopeWithKeyManager(se.GetKeyManager(), 1000)

	decrypted1, err := se2.Decrypt(envelope1)
	if err != nil {
		t.Fatalf("Decrypt v1 message failed: %v", err)
	}
	if !bytes.Equal(plaintext1, decrypted1) {
		t.Errorf("v1 decrypted data mismatch")
	}

	decrypted2, err := se2.Decrypt(envelope2)
	if err != nil {
		t.Fatalf("Decrypt v2 message failed: %v", err)
	}
	if !bytes.Equal(plaintext2, decrypted2) {
		t.Errorf("v2 decrypted data mismatch")
	}
}

func TestKeyPruning(t *testing.T) {
	cfg := &Config{
		MaxKeys: 3,
	}
	km := NewKeyManager(cfg)

	for i := uint32(1); i <= 5; i++ {
		kv, err := NewKeyVersion(i)
		if err != nil {
			t.Fatalf("NewKeyVersion %d failed: %v", i, err)
		}
		if err := km.AddKey(kv); err != nil {
			t.Fatalf("AddKey %d failed: %v", i, err)
		}
	}

	if km.KeyCount() != 3 {
		t.Errorf("expected 3 keys after pruning, got %d", km.KeyCount())
	}

	if km.CurrentVersion() != 5 {
		t.Errorf("expected current version 5, got %d", km.CurrentVersion())
	}

	_, exists := km.GetKey(1)
	if exists {
		t.Error("key version 1 should have been pruned")
	}

	_, exists = km.GetKey(2)
	if exists {
		t.Error("key version 2 should have been pruned")
	}

	_, exists = km.GetKey(3)
	if !exists {
		t.Error("key version 3 should exist")
	}

	_, exists = km.GetKey(5)
	if !exists {
		t.Error("key version 5 should exist")
	}
}

func TestRotateKeyWithPruning(t *testing.T) {
	cfg := &Config{
		MaxKeys: 3,
	}
	se, err := NewSecureEnvelope(cfg)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		_, err := se.RotateKey()
		if err != nil {
			t.Fatalf("RotateKey %d failed: %v", i, err)
		}
	}

	if se.keyManager.KeyCount() != 3 {
		t.Errorf("expected 3 keys, got %d", se.keyManager.KeyCount())
	}
}

func TestNewKeyVersionWithKeys(t *testing.T) {
	encKey := make([]byte, KeySize)
	signKey := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, encKey); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(rand.Reader, signKey); err != nil {
		t.Fatal(err)
	}

	kv, err := NewKeyVersionWithKeys(1, encKey, signKey)
	if err != nil {
		t.Fatalf("NewKeyVersionWithKeys failed: %v", err)
	}
	if !bytes.Equal(kv.EncryptKey, encKey) {
		t.Error("encrypt key mismatch")
	}
	if !bytes.Equal(kv.SignKey, signKey) {
		t.Error("sign key mismatch")
	}

	kv.EncryptKey[0] ^= 0xFF
	if bytes.Equal(kv.EncryptKey, encKey) {
		t.Error("key should be copied, not referenced")
	}
}

func TestNewKeyVersionWithKeysInvalidSize(t *testing.T) {
	shortKey := make([]byte, 16)
	_, err := NewKeyVersionWithKeys(1, shortKey, make([]byte, KeySize))
	if err != ErrInvalidKeySize {
		t.Errorf("expected ErrInvalidKeySize for short encrypt key, got %v", err)
	}

	_, err = NewKeyVersionWithKeys(1, make([]byte, KeySize), shortKey)
	if err != ErrInvalidKeySize {
		t.Errorf("expected ErrInvalidKeySize for short sign key, got %v", err)
	}
}

func TestGetKeyReturnsCopy(t *testing.T) {
	km := NewKeyManager(nil)
	kv, err := NewKeyVersion(1)
	if err != nil {
		t.Fatalf("NewKeyVersion failed: %v", err)
	}
	originalEnc := make([]byte, KeySize)
	copy(originalEnc, kv.EncryptKey)
	if err := km.AddKey(kv); err != nil {
		t.Fatalf("AddKey failed: %v", err)
	}

	retrieved, ok := km.GetKey(1)
	if !ok {
		t.Fatal("GetKey failed")
	}

	retrieved.EncryptKey[0] ^= 0xFF

	retrieved2, _ := km.GetKey(1)
	if !bytes.Equal(retrieved2.EncryptKey, originalEnc) {
		t.Error("GetKey should return a copy of the key")
	}
}

func TestSequenceNumberIncrement(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	if se.CurrentSequence() != 0 {
		t.Errorf("expected initial sequence 0, got %d", se.CurrentSequence())
	}

	plaintext := []byte("test")
	for i := uint64(1); i <= 5; i++ {
		_, err := se.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt %d failed: %v", i, err)
		}
		if se.CurrentSequence() != i {
			t.Errorf("expected sequence %d, got %d", i, se.CurrentSequence())
		}
	}
}

func TestVerifyMethod(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("verify test")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	err = se.Verify(envelope)
	if err != nil {
		t.Errorf("Verify should succeed, got %v", err)
	}

	envelope[len(envelope)-1] ^= 0xFF
	err = se.Verify(envelope)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestVerifyEmptyData(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	err = se.Verify([]byte{})
	if err != ErrEmptyData {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestMultipleMessages(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	messages := [][]byte{
		[]byte("message 1"),
		[]byte("message 2"),
		[]byte("message 3"),
		[]byte("a much longer message that should still work correctly with the encryption scheme"),
		[]byte("short"),
	}

	envelopes := make([][]byte, len(messages))
	for i, msg := range messages {
		env, err := se.Encrypt(msg)
		if err != nil {
			t.Fatalf("Encrypt %d failed: %v", i, err)
		}
		envelopes[i] = env
	}

	se2 := NewSecureEnvelopeWithKeyManager(se.GetKeyManager(), 1000)
	for i, env := range envelopes {
		decrypted, err := se2.Decrypt(env)
		if err != nil {
			t.Fatalf("Decrypt %d failed: %v", i, err)
		}
		if !bytes.Equal(messages[i], decrypted) {
			t.Errorf("message %d mismatch", i)
		}
	}
}

func TestLargeData(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	largeData := make([]byte, 1024*1024)
	if _, err := io.ReadFull(rand.Reader, largeData); err != nil {
		t.Fatalf("generate random data failed: %v", err)
	}

	envelope, err := se.Encrypt(largeData)
	if err != nil {
		t.Fatalf("Encrypt large data failed: %v", err)
	}

	decrypted, err := se.Decrypt(envelope)
	if err != nil {
		t.Fatalf("Decrypt large data failed: %v", err)
	}

	if !bytes.Equal(largeData, decrypted) {
		t.Error("large data decryption mismatch")
	}

	hashOrig := sha256.Sum256(largeData)
	hashDec := sha256.Sum256(decrypted)
	if hashOrig != hashDec {
		t.Error("hash mismatch for large data")
	}
}

func TestConcurrentEncrypt(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			plaintext := []byte("concurrent message")
			_, err := se.Encrypt(plaintext)
			if err != nil {
				t.Errorf("goroutine %d: Encrypt failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	if se.CurrentSequence() != uint64(numGoroutines) {
		t.Errorf("expected sequence %d, got %d", numGoroutines, se.CurrentSequence())
	}
}

func TestConcurrentDecrypt(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	km := se.GetKeyManager()
	seEncrypt := NewSecureEnvelopeWithKeyManager(km, 1000)

	numMessages := 100
	envelopes := make([][]byte, numMessages)
	for i := 0; i < numMessages; i++ {
		plaintext := []byte("concurrent decrypt test")
		env, err := seEncrypt.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt %d failed: %v", i, err)
		}
		envelopes[i] = env
	}

	var wg sync.WaitGroup
	for i, env := range envelopes {
		wg.Add(1)
		go func(id int, envelope []byte) {
			defer wg.Done()
			seDecrypt := NewSecureEnvelopeWithKeyManager(km, 1000)
			_, err := seDecrypt.Decrypt(envelope)
			if err != nil {
				t.Errorf("goroutine %d: Decrypt failed: %v", id, err)
			}
		}(i, env)
	}

	wg.Wait()
}

func TestAddNilKey(t *testing.T) {
	km := NewKeyManager(nil)
	err := km.AddKey(nil)
	if err == nil {
		t.Error("expected error when adding nil key")
	}
}

func TestAADIntegrity(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("AAD test")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	seqOffset := 5
	envelope[seqOffset] ^= 0xFF

	_, err = se.Decrypt(envelope)
	if err != ErrInvalidSignature {
		t.Errorf("tampering with sequence number (AAD) should be detected by HMAC, got %v", err)
	}
}

func TestAADNonceIntegrity(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("AAD nonce test")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	nonceOffset := 1 + 4 + 8
	envelope[nonceOffset] ^= 0xFF

	_, err = se.Decrypt(envelope)
	if err != ErrInvalidSignature {
		t.Errorf("tampering with nonce (AAD) should be detected by HMAC, got %v", err)
	}
}

func TestEnvelopeSize(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	testCases := []int{1, 16, 64, 256, 1024}
	for _, size := range testCases {
		plaintext := make([]byte, size)
		envelope, err := se.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt size %d failed: %v", size, err)
		}

		expectedSize := 1 + 4 + 8 + NonceSize + size + GCMTagSize + HMACSize
		if len(envelope) != expectedSize {
			t.Errorf("size %d: expected envelope size %d, got %d", size, expectedSize, len(envelope))
		}
	}
}

func TestKeyManagerWithZeroMaxKeys(t *testing.T) {
	cfg := &Config{
		MaxKeys: 0,
	}
	km := NewKeyManager(cfg)
	if km.MaxKeys() != DefaultMaxKeys {
		t.Errorf("expected default max keys %d, got %d", DefaultMaxKeys, km.MaxKeys())
	}
}

func TestReplayProtectorWithZeroWindow(t *testing.T) {
	rp := NewReplayProtector(0)
	if rp.windowSize != DefaultReplayWin {
		t.Errorf("expected default window %d, got %d", DefaultReplayWin, rp.windowSize)
	}
}

func TestGetMaxSequence(t *testing.T) {
	rp := NewReplayProtector(100)

	if rp.GetMaxSequence(1) != 0 {
		t.Errorf("expected max sequence 0 for unseen version, got %d", rp.GetMaxSequence(1))
	}

	err := rp.CheckAndUpdate(1, 42)
	if err != nil {
		t.Fatalf("CheckAndUpdate failed: %v", err)
	}

	if rp.GetMaxSequence(1) != 42 {
		t.Errorf("expected max sequence 42, got %d", rp.GetMaxSequence(1))
	}
}

func TestDecryptWithReplayProtectorState(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("test")
	envelope1, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1 failed: %v", err)
	}

	envelope2, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2 failed: %v", err)
	}

	se2 := NewSecureEnvelopeWithKeyManager(se.GetKeyManager(), 1000)

	_, err = se2.Decrypt(envelope2)
	if err != nil {
		t.Fatalf("Decrypt 2 should succeed: %v", err)
	}

	_, err = se2.Decrypt(envelope1)
	if err != ErrReplayDetected {
		t.Errorf("Decrypt 1 after 2 should fail with replay, got %v", err)
	}
}

func TestNewSecureEnvelopeWithKeyManager(t *testing.T) {
	km := NewKeyManager(nil)
	kv, err := NewKeyVersion(1)
	if err != nil {
		t.Fatalf("NewKeyVersion failed: %v", err)
	}
	if err := km.AddKey(kv); err != nil {
		t.Fatalf("AddKey failed: %v", err)
	}

	se := NewSecureEnvelopeWithKeyManager(km, 500)
	if se == nil {
		t.Fatal("NewSecureEnvelopeWithKeyManager returned nil")
	}
	if se.GetKeyManager() != km {
		t.Error("key manager mismatch")
	}
	if se.replayGuard.windowSize != 500 {
		t.Errorf("expected window size 500, got %d", se.replayGuard.windowSize)
	}
}

func TestBinaryEncodingConsistency(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	plaintext := []byte("binary encoding test")
	envelope, err := se.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	keyVersion := binary.BigEndian.Uint32(envelope[1:5])
	if keyVersion != 1 {
		t.Errorf("expected key version 1, got %d", keyVersion)
	}

	seqNum := binary.BigEndian.Uint64(envelope[5:13])
	if seqNum != 1 {
		t.Errorf("expected sequence number 1, got %d", seqNum)
	}
}

func TestKeyVersionTimestamp(t *testing.T) {
	before := time.Now()
	time.Sleep(time.Millisecond)
	kv, err := NewKeyVersion(1)
	if err != nil {
		t.Fatalf("NewKeyVersion failed: %v", err)
	}
	time.Sleep(time.Millisecond)
	after := time.Now()

	if kv.CreatedAt.Before(before) || kv.CreatedAt.After(after) {
		t.Error("KeyVersion CreatedAt timestamp is not within expected range")
	}
}

func TestNonceUniqueness(t *testing.T) {
	se, err := NewSecureEnvelope(nil)
	if err != nil {
		t.Fatalf("NewSecureEnvelope failed: %v", err)
	}

	nonces := make(map[string]bool)
	plaintext := []byte("test")

	for i := 0; i < 100; i++ {
		envelope, err := se.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt %d failed: %v", i, err)
		}

		nonceStart := 1 + 4 + 8
		nonce := string(envelope[nonceStart : nonceStart+NonceSize])
		if nonces[nonce] {
			t.Errorf("duplicate nonce detected at iteration %d", i)
		}
		nonces[nonce] = true
	}
}
