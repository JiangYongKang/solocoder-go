package lsm

import (
	"bytes"
	"encoding/binary"
)

type Entry struct {
	Key       string
	Value     string
	Tombstone bool
	Timestamp int64
}

func (e *Entry) Size() int {
	return len(e.Key) + len(e.Value) + 1 + 8
}

func (e *Entry) Encode() []byte {
	buf := new(bytes.Buffer)

	keyLen := uint32(len(e.Key))
	binary.Write(buf, binary.LittleEndian, keyLen)
	buf.WriteString(e.Key)

	valLen := uint32(len(e.Value))
	binary.Write(buf, binary.LittleEndian, valLen)
	buf.WriteString(e.Value)

	tombstone := byte(0)
	if e.Tombstone {
		tombstone = 1
	}
	buf.WriteByte(tombstone)

	binary.Write(buf, binary.LittleEndian, e.Timestamp)

	return buf.Bytes()
}

func DecodeEntry(data []byte) (*Entry, int, error) {
	buf := bytes.NewReader(data)
	e := &Entry{}

	var keyLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &keyLen); err != nil {
		return nil, 0, err
	}

	keyBytes := make([]byte, keyLen)
	if _, err := buf.Read(keyBytes); err != nil {
		return nil, 0, err
	}
	e.Key = string(keyBytes)

	var valLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &valLen); err != nil {
		return nil, 0, err
	}

	valBytes := make([]byte, valLen)
	if _, err := buf.Read(valBytes); err != nil {
		return nil, 0, err
	}
	e.Value = string(valBytes)

	tombstone, err := buf.ReadByte()
	if err != nil {
		return nil, 0, err
	}
	e.Tombstone = tombstone == 1

	if err := binary.Read(buf, binary.LittleEndian, &e.Timestamp); err != nil {
		return nil, 0, err
	}

	read := 4 + int(keyLen) + 4 + int(valLen) + 1 + 8
	return e, read, nil
}

type IndexEntry struct {
	Key      string
	Offset   int64
	EntryLen int32
}

func (ie *IndexEntry) Encode() []byte {
	buf := new(bytes.Buffer)

	keyLen := uint32(len(ie.Key))
	binary.Write(buf, binary.LittleEndian, keyLen)
	buf.WriteString(ie.Key)

	binary.Write(buf, binary.LittleEndian, ie.Offset)
	binary.Write(buf, binary.LittleEndian, ie.EntryLen)

	return buf.Bytes()
}

func DecodeIndexEntry(data []byte) (*IndexEntry, int, error) {
	buf := bytes.NewReader(data)
	ie := &IndexEntry{}

	var keyLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &keyLen); err != nil {
		return nil, 0, err
	}

	keyBytes := make([]byte, keyLen)
	if _, err := buf.Read(keyBytes); err != nil {
		return nil, 0, err
	}
	ie.Key = string(keyBytes)

	if err := binary.Read(buf, binary.LittleEndian, &ie.Offset); err != nil {
		return nil, 0, err
	}

	if err := binary.Read(buf, binary.LittleEndian, &ie.EntryLen); err != nil {
		return nil, 0, err
	}

	read := 4 + int(keyLen) + 8 + 4
	return ie, read, nil
}
