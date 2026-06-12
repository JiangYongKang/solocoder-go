package secenv

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	FormatVersion    byte   = 1
	KeySize          int    = 32
	NonceSize        int    = 12
	GCMTagSize       int    = 16
	HMACSize         int    = 32
	HeaderSize       int    = 1 + 4 + 8 + NonceSize
	DefaultMaxKeys   int    = 10
	DefaultReplayWin uint64 = 1000
)

var (
	ErrInvalidFormat    = errors.New("secenv: invalid envelope format")
	ErrInvalidVersion   = errors.New("secenv: unsupported format version")
	ErrKeyNotFound      = errors.New("secenv: key version not found")
	ErrInvalidSignature = errors.New("secenv: invalid HMAC signature")
	ErrInvalidTag       = errors.New("secenv: invalid GCM authentication tag")
	ErrReplayDetected   = errors.New("secenv: replay attack detected")
	ErrInvalidKeySize   = errors.New("secenv: invalid key size, must be 32 bytes")
	ErrEmptyData        = errors.New("secenv: empty data")
)

type KeyVersion struct {
	Version      uint32
	EncryptKey   []byte
	SignKey      []byte
	CreatedAt    time.Time
}

type Envelope struct {
	FormatVersion  byte
	KeyVersion     uint32
	SequenceNum    uint64
	Nonce          []byte
	Ciphertext     []byte
	GCMTag         []byte
	HMACSignature  []byte
}

type KeyManager struct {
	keys        map[uint32]*KeyVersion
	currentVer  uint32
	maxKeys     int
	mu          sync.RWMutex
}

type ReplayProtector struct {
	seen        map[uint32]uint64
	windowSize  uint64
	mu          sync.Mutex
}

type SecureEnvelope struct {
	keyManager    *KeyManager
	replayGuard   *ReplayProtector
	sequenceNum   uint64
	mu            sync.Mutex
}

type Config struct {
	MaxKeys       int
	ReplayWindow  uint64
}

func DefaultConfig() *Config {
	return &Config{
		MaxKeys:      DefaultMaxKeys,
		ReplayWindow: DefaultReplayWin,
	}
}

func NewKeyVersion(version uint32) (*KeyVersion, error) {
	encKey := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, encKey); err != nil {
		return nil, fmt.Errorf("secenv: generate encrypt key: %w", err)
	}

	signKey := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, signKey); err != nil {
		return nil, fmt.Errorf("secenv: generate sign key: %w", err)
	}

	return &KeyVersion{
		Version:    version,
		EncryptKey: encKey,
		SignKey:    signKey,
		CreatedAt:  time.Now(),
	}, nil
}

func NewKeyVersionWithKeys(version uint32, encKey, signKey []byte) (*KeyVersion, error) {
	if len(encKey) != KeySize {
		return nil, ErrInvalidKeySize
	}
	if len(signKey) != KeySize {
		return nil, ErrInvalidKeySize
	}

	encCopy := make([]byte, KeySize)
	copy(encCopy, encKey)

	signCopy := make([]byte, KeySize)
	copy(signCopy, signKey)

	return &KeyVersion{
		Version:    version,
		EncryptKey: encCopy,
		SignKey:    signCopy,
		CreatedAt:  time.Now(),
	}, nil
}

func NewKeyManager(cfg *Config) *KeyManager {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	km := &KeyManager{
		keys:       make(map[uint32]*KeyVersion),
		currentVer: 0,
		maxKeys:    cfg.MaxKeys,
	}

	if km.maxKeys <= 0 {
		km.maxKeys = DefaultMaxKeys
	}

	return km
}

func (km *KeyManager) AddKey(kv *KeyVersion) error {
	if kv == nil {
		return errors.New("secenv: nil key version")
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	km.keys[kv.Version] = kv
	if kv.Version > km.currentVer {
		km.currentVer = kv.Version
	}

	km.pruneOldKeys()
	return nil
}

func (km *KeyManager) RotateKey() (*KeyVersion, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	newVer := km.currentVer + 1
	kv, err := NewKeyVersion(newVer)
	if err != nil {
		return nil, err
	}

	km.keys[newVer] = kv
	km.currentVer = newVer

	km.pruneOldKeys()
	return kv, nil
}

func (km *KeyManager) pruneOldKeys() {
	if len(km.keys) <= km.maxKeys {
		return
	}

	versions := make([]uint32, 0, len(km.keys))
	for v := range km.keys {
		versions = append(versions, v)
	}

	for i := 0; i < len(versions)-1; i++ {
		for j := i + 1; j < len(versions); j++ {
			if versions[i] > versions[j] {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}

	removeCount := len(versions) - km.maxKeys
	for i := 0; i < removeCount; i++ {
		delete(km.keys, versions[i])
	}
}

func (km *KeyManager) GetKey(version uint32) (*KeyVersion, bool) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	kv, ok := km.keys[version]
	if !ok {
		return nil, false
	}

	encCopy := make([]byte, KeySize)
	copy(encCopy, kv.EncryptKey)

	signCopy := make([]byte, KeySize)
	copy(signCopy, kv.SignKey)

	return &KeyVersion{
		Version:    kv.Version,
		EncryptKey: encCopy,
		SignKey:    signCopy,
		CreatedAt:  kv.CreatedAt,
	}, true
}

func (km *KeyManager) CurrentVersion() uint32 {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.currentVer
}

func (km *KeyManager) KeyCount() int {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return len(km.keys)
}

func (km *KeyManager) MaxKeys() int {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.maxKeys
}

func NewReplayProtector(windowSize uint64) *ReplayProtector {
	if windowSize <= 0 {
		windowSize = DefaultReplayWin
	}

	return &ReplayProtector{
		seen:       make(map[uint32]uint64),
		windowSize: windowSize,
	}
}

func (rp *ReplayProtector) CheckAndUpdate(keyVersion uint32, seq uint64) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	maxSeq, exists := rp.seen[keyVersion]
	if !exists {
		rp.seen[keyVersion] = seq
		return nil
	}

	if seq <= maxSeq {
		return ErrReplayDetected
	}

	if seq >= maxSeq+rp.windowSize {
		return ErrReplayDetected
	}

	rp.seen[keyVersion] = seq
	return nil
}

func (rp *ReplayProtector) GetMaxSequence(keyVersion uint32) uint64 {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return rp.seen[keyVersion]
}

func (rp *ReplayProtector) Reset() {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.seen = make(map[uint32]uint64)
}

func NewSecureEnvelope(cfg *Config) (*SecureEnvelope, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	km := NewKeyManager(cfg)
	kv, err := NewKeyVersion(1)
	if err != nil {
		return nil, err
	}

	if err := km.AddKey(kv); err != nil {
		return nil, err
	}

	return &SecureEnvelope{
		keyManager:  km,
		replayGuard: NewReplayProtector(cfg.ReplayWindow),
		sequenceNum: 0,
	}, nil
}

func NewSecureEnvelopeWithKeyManager(km *KeyManager, replayWindow uint64) *SecureEnvelope {
	return &SecureEnvelope{
		keyManager:  km,
		replayGuard: NewReplayProtector(replayWindow),
		sequenceNum: 0,
	}
}

func (se *SecureEnvelope) Encrypt(plaintext []byte) ([]byte, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	kv, ok := se.keyManager.GetKey(se.keyManager.CurrentVersion())
	if !ok {
		return nil, ErrKeyNotFound
	}

	se.sequenceNum++
	seq := se.sequenceNum

	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("secenv: generate nonce: %w", err)
	}

	block, err := aes.NewCipher(kv.EncryptKey)
	if err != nil {
		return nil, fmt.Errorf("secenv: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secenv: create GCM: %w", err)
	}

	aad := se.buildAAD(kv.Version, seq, nonce)

	ciphertextWithTag := gcm.Seal(nil, nonce, plaintext, aad)
	ciphertext := ciphertextWithTag[:len(ciphertextWithTag)-GCMTagSize]
	gcmTag := ciphertextWithTag[len(ciphertextWithTag)-GCMTagSize:]

	env := &Envelope{
		FormatVersion: FormatVersion,
		KeyVersion:    kv.Version,
		SequenceNum:   seq,
		Nonce:         nonce,
		Ciphertext:    ciphertext,
		GCMTag:        gcmTag,
	}

	signature, err := se.computeHMAC(env, kv.SignKey)
	if err != nil {
		return nil, err
	}
	env.HMACSignature = signature

	return se.serialize(env)
}

func (se *SecureEnvelope) Decrypt(envelopeBytes []byte) ([]byte, error) {
	if len(envelopeBytes) == 0 {
		return nil, ErrEmptyData
	}

	env, err := se.deserialize(envelopeBytes)
	if err != nil {
		return nil, err
	}

	kv, ok := se.keyManager.GetKey(env.KeyVersion)
	if !ok {
		return nil, ErrKeyNotFound
	}

	expectedSig, err := se.computeHMAC(env, kv.SignKey)
	if err != nil {
		return nil, err
	}

	if !hmac.Equal(env.HMACSignature, expectedSig) {
		return nil, ErrInvalidSignature
	}

	if err := se.replayGuard.CheckAndUpdate(env.KeyVersion, env.SequenceNum); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(kv.EncryptKey)
	if err != nil {
		return nil, fmt.Errorf("secenv: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secenv: create GCM: %w", err)
	}

	aad := se.buildAAD(env.KeyVersion, env.SequenceNum, env.Nonce)
	ciphertextWithTag := append(env.Ciphertext, env.GCMTag...)

	plaintext, err := gcm.Open(nil, env.Nonce, ciphertextWithTag, aad)
	if err != nil {
		return nil, ErrInvalidTag
	}

	return plaintext, nil
}

func (se *SecureEnvelope) Verify(envelopeBytes []byte) error {
	if len(envelopeBytes) == 0 {
		return ErrEmptyData
	}

	env, err := se.deserialize(envelopeBytes)
	if err != nil {
		return err
	}

	kv, ok := se.keyManager.GetKey(env.KeyVersion)
	if !ok {
		return ErrKeyNotFound
	}

	expectedSig, err := se.computeHMAC(env, kv.SignKey)
	if err != nil {
		return err
	}

	if !hmac.Equal(env.HMACSignature, expectedSig) {
		return ErrInvalidSignature
	}

	if err := se.replayGuard.CheckAndUpdate(env.KeyVersion, env.SequenceNum); err != nil {
		return err
	}

	return nil
}

func (se *SecureEnvelope) RotateKey() (*KeyVersion, error) {
	return se.keyManager.RotateKey()
}

func (se *SecureEnvelope) AddKey(kv *KeyVersion) error {
	return se.keyManager.AddKey(kv)
}

func (se *SecureEnvelope) GetKeyManager() *KeyManager {
	return se.keyManager
}

func (se *SecureEnvelope) GetReplayProtector() *ReplayProtector {
	return se.replayGuard
}

func (se *SecureEnvelope) CurrentSequence() uint64 {
	se.mu.Lock()
	defer se.mu.Unlock()
	return se.sequenceNum
}

func (se *SecureEnvelope) buildAAD(keyVer uint32, seq uint64, nonce []byte) []byte {
	aad := make([]byte, 1+4+8+NonceSize)
	aad[0] = FormatVersion
	binary.BigEndian.PutUint32(aad[1:5], keyVer)
	binary.BigEndian.PutUint64(aad[5:13], seq)
	copy(aad[13:13+NonceSize], nonce)
	return aad
}

func (se *SecureEnvelope) computeHMAC(env *Envelope, signKey []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, signKey)

	mac.Write([]byte{env.FormatVersion})

	verBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(verBuf, env.KeyVersion)
	mac.Write(verBuf)

	seqBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(seqBuf, env.SequenceNum)
	mac.Write(seqBuf)

	mac.Write(env.Nonce)
	mac.Write(env.Ciphertext)
	mac.Write(env.GCMTag)

	return mac.Sum(nil), nil
}

func (se *SecureEnvelope) serialize(env *Envelope) ([]byte, error) {
	totalSize := 1 + 4 + 8 + NonceSize + len(env.Ciphertext) + GCMTagSize + HMACSize
	buf := make([]byte, totalSize)

	offset := 0
	buf[offset] = env.FormatVersion
	offset++

	binary.BigEndian.PutUint32(buf[offset:offset+4], env.KeyVersion)
	offset += 4

	binary.BigEndian.PutUint64(buf[offset:offset+8], env.SequenceNum)
	offset += 8

	copy(buf[offset:offset+NonceSize], env.Nonce)
	offset += NonceSize

	copy(buf[offset:offset+len(env.Ciphertext)], env.Ciphertext)
	offset += len(env.Ciphertext)

	copy(buf[offset:offset+GCMTagSize], env.GCMTag)
	offset += GCMTagSize

	copy(buf[offset:offset+HMACSize], env.HMACSignature)
	offset += HMACSize

	return buf, nil
}

func (se *SecureEnvelope) deserialize(data []byte) (*Envelope, error) {
	minSize := 1 + 4 + 8 + NonceSize + GCMTagSize + HMACSize
	if len(data) < minSize {
		return nil, ErrInvalidFormat
	}

	env := &Envelope{}

	offset := 0
	env.FormatVersion = data[offset]
	offset++

	if env.FormatVersion != FormatVersion {
		return nil, ErrInvalidVersion
	}

	env.KeyVersion = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	env.SequenceNum = binary.BigEndian.Uint64(data[offset : offset+8])
	offset += 8

	env.Nonce = make([]byte, NonceSize)
	copy(env.Nonce, data[offset:offset+NonceSize])
	offset += NonceSize

	remaining := len(data) - offset
	if remaining < GCMTagSize+HMACSize {
		return nil, ErrInvalidFormat
	}

	ciphertextLen := remaining - GCMTagSize - HMACSize
	env.Ciphertext = make([]byte, ciphertextLen)
	copy(env.Ciphertext, data[offset:offset+ciphertextLen])
	offset += ciphertextLen

	env.GCMTag = make([]byte, GCMTagSize)
	copy(env.GCMTag, data[offset:offset+GCMTagSize])
	offset += GCMTagSize

	env.HMACSignature = make([]byte, HMACSize)
	copy(env.HMACSignature, data[offset:offset+HMACSize])
	offset += HMACSize

	return env, nil
}
