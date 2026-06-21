package chainhash

import "errors"

var (
	ErrNodeNotFound      = errors.New("chainhash: node not found")
	ErrNodeAlreadyExists = errors.New("chainhash: node already exists")
	ErrEmptyRing         = errors.New("chainhash: hash ring is empty")
	ErrInvalidWeight     = errors.New("chainhash: invalid weight, must be positive")
	ErrInvalidVirtualNodes = errors.New("chainhash: invalid virtual nodes, must be positive")
	ErrEmptyNodeID       = errors.New("chainhash: node id cannot be empty")
	ErrSerializationFailed = errors.New("chainhash: serialization failed")
	ErrDeserializationFailed = errors.New("chainhash: deserialization failed")
	ErrFileIO            = errors.New("chainhash: file I/O error")
)
