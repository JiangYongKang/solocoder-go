package datadedup

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	magicNumber   uint32 = 0x44445550
	version       uint16 = 1
	entryTypeFP   byte   = 1
	entryTypeCS   byte   = 2
)

type persistIndex struct {
	mu sync.Mutex
}

func NewPersistIndex() PersistIndex {
	return &persistIndex{}
}

type persistHeader struct {
	Magic    uint32
	Version  uint16
	Reserved uint16
	Count    uint64
}

type persistEntry struct {
	Type      byte
	FP        Fingerprint
	Timestamp int64
}

func (p *persistIndex) Save(index FingerprintIndex, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.saveLocked(index, path)
}

func (p *persistIndex) saveLocked(index FingerprintIndex, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	writer := bufio.NewWriter(f)

	checksum := sha256.New()
	mw := io.MultiWriter(writer, checksum)

	header := persistHeader{
		Magic:    magicNumber,
		Version:  version,
		Reserved: 0,
		Count:    uint64(len(index)),
	}

	headerBytes := make([]byte, binary.Size(header))
	binary.BigEndian.PutUint32(headerBytes[0:4], header.Magic)
	binary.BigEndian.PutUint16(headerBytes[4:6], header.Version)
	binary.BigEndian.PutUint16(headerBytes[6:8], header.Reserved)
	binary.BigEndian.PutUint64(headerBytes[8:16], header.Count)

	if _, err := mw.Write(headerBytes); err != nil {
		return err
	}

	for fp := range index {
		entry := persistEntry{
			Type:      entryTypeFP,
			FP:        fp,
			Timestamp: 0,
		}

		entryBytes := encodeEntry(entry)
		if _, err := mw.Write(entryBytes); err != nil {
			return err
		}
	}

	cs := hex.EncodeToString(checksum.Sum(nil))

	csEntry := persistEntry{
		Type:      entryTypeCS,
		FP:        Fingerprint(cs),
		Timestamp: 0,
	}

	csEntryBytes := encodeEntry(csEntry)
	if _, err := writer.Write(csEntryBytes); err != nil {
		return err
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

func (p *persistIndex) Load(path string) (FingerprintIndex, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.loadLocked(path)
}

func (p *persistIndex) loadLocked(path string) (FingerprintIndex, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrPersistFileNotExist
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) < binary.Size(persistHeader{}) {
		return nil, ErrPersistCorrupted
	}

	header := persistHeader{
		Magic:    binary.BigEndian.Uint32(data[0:4]),
		Version:  binary.BigEndian.Uint16(data[4:6]),
		Reserved: binary.BigEndian.Uint16(data[6:8]),
		Count:    binary.BigEndian.Uint64(data[8:16]),
	}

	if header.Magic != magicNumber {
		return nil, ErrPersistCorrupted
	}

	if header.Version != version {
		return nil, ErrPersistCorrupted
	}

	offset := binary.Size(header)
	checksum := sha256.New()
	checksum.Write(data[0:offset])

	index := make(FingerprintIndex, header.Count)
	var lastChecksum string
	fpCount := uint64(0)

	for offset < len(data) {
		if offset+1 > len(data) {
			return nil, ErrPersistCorrupted
		}
		entryType := data[offset]

		if offset+5 > len(data) {
			return nil, ErrPersistCorrupted
		}
		fpLen := binary.BigEndian.Uint32(data[offset+1 : offset+5])

		if fpLen > 1024 {
			return nil, ErrPersistCorrupted
		}

		if offset+5+int(fpLen)+8 > len(data) {
			return nil, ErrPersistCorrupted
		}

		fpBytes := data[offset+5 : offset+5+int(fpLen)]

		entryLen := 1 + 4 + int(fpLen) + 8

		if entryType == entryTypeCS {
			lastChecksum = string(fpBytes)
			offset += entryLen
			break
		}

		if entryType == entryTypeFP {
			checksum.Write(data[offset : offset+entryLen])
			index[Fingerprint(fpBytes)] = true
			fpCount++
		}

		offset += entryLen
	}

	if lastChecksum == "" {
		return nil, ErrPersistCorrupted
	}

	if fpCount != header.Count {
		return nil, ErrPersistCorrupted
	}

	calculated := hex.EncodeToString(checksum.Sum(nil))
	if calculated != lastChecksum {
		return nil, ErrChecksumMismatch
	}

	return index, nil
}

func (p *persistIndex) Append(fp Fingerprint, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return p.saveLocked(FingerprintIndex{fp: true}, path)
	}

	index, err := p.loadLocked(path)
	if err != nil {
		return err
	}

	index[fp] = true

	return p.saveLocked(index, path)
}

func (p *persistIndex) Verify(path string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, ErrPersistFileNotExist
	}

	_, err := p.loadLocked(path)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func encodeEntry(entry persistEntry) []byte {
	fpBytes := []byte(entry.FP)
	fpLen := uint32(len(fpBytes))

	buf := make([]byte, 1+4+int(fpLen)+8)
	buf[0] = entry.Type
	binary.BigEndian.PutUint32(buf[1:5], fpLen)
	copy(buf[5:5+int(fpLen)], fpBytes)
	binary.BigEndian.PutUint64(buf[5+int(fpLen):], uint64(entry.Timestamp))

	return buf
}

type indexPersister interface {
	GetIndex() FingerprintIndex
	SetIndex(index FingerprintIndex)
	AddFingerprint(fp Fingerprint)
	Count() int
	Close() error
}

func saveIndex(persister indexPersister, persisterImpl PersistIndex, path string) error {
	index := persister.GetIndex()
	return persisterImpl.Save(index, path)
}

func loadIndex(persister indexPersister, persisterImpl PersistIndex, path string) error {
	index, err := persisterImpl.Load(path)
	if err != nil {
		return err
	}
	persister.SetIndex(index)
	return nil
}

func appendFingerprint(persister indexPersister, persisterImpl PersistIndex, fp Fingerprint, path string) error {
	if err := persisterImpl.Append(fp, path); err != nil {
		return err
	}
	persister.AddFingerprint(fp)
	return nil
}

func verifyIndex(persisterImpl PersistIndex, path string) (bool, error) {
	return persisterImpl.Verify(path)
}
