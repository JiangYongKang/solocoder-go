package bplustree

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrKeyNotFound      = errors.New("key not found")
	ErrInvalidRange     = errors.New("invalid range: start > end")
	ErrInvalidMaxKeys   = errors.New("invalid max keys: must be >= 2")
	ErrIteratorInvalid  = errors.New("iterator is invalid")
	ErrIteratorDone     = errors.New("iterator has no more elements")
)

type Config struct {
	MaxKeys int
}

func DefaultConfig() Config {
	return Config{
		MaxKeys: 32,
	}
}

type KVItem struct {
	Key   string
	Value string
}

type node struct {
	keys     []string
	isLeaf   bool
	parent   *node
	next     *node
	prev     *node
	children []*node
	values   []string
}

type BPlusTree struct {
	root    *node
	maxKeys int
	count   int
}

func NewBPlusTree() *BPlusTree {
	return NewBPlusTreeWithConfig(DefaultConfig())
}

func NewBPlusTreeWithConfig(cfg Config) *BPlusTree {
	if cfg.MaxKeys < 2 {
		cfg.MaxKeys = 32
	}
	if cfg.MaxKeys%2 != 0 {
		cfg.MaxKeys++
	}

	root := &node{
		isLeaf: true,
		keys:   make([]string, 0, cfg.MaxKeys),
		values: make([]string, 0, cfg.MaxKeys),
	}

	return &BPlusTree{
		root:    root,
		maxKeys: cfg.MaxKeys,
		count:   0,
	}
}

func (t *BPlusTree) Count() int {
	return t.count
}

func (t *BPlusTree) Insert(key string, value string) {
	leaf := t.findLeaf(key)

	idx := 0
	for idx < len(leaf.keys) && leaf.keys[idx] < key {
		idx++
	}

	if idx < len(leaf.keys) && leaf.keys[idx] == key {
		leaf.values[idx] = value
		return
	}

	leaf.keys = append(leaf.keys, "")
	leaf.values = append(leaf.values, "")
	for i := len(leaf.keys) - 1; i > idx; i-- {
		leaf.keys[i] = leaf.keys[i-1]
		leaf.values[i] = leaf.values[i-1]
	}
	leaf.keys[idx] = key
	leaf.values[idx] = value

	t.count++

	if len(leaf.keys) > t.maxKeys {
		t.splitLeaf(leaf)
	}
}

func (t *BPlusTree) findLeaf(key string) *node {
	current := t.root
	for !current.isLeaf {
		idx := 0
		for idx < len(current.keys) && key >= current.keys[idx] {
			idx++
		}
		current = current.children[idx]
	}
	return current
}

func (t *BPlusTree) splitLeaf(leaf *node) {
	mid := len(leaf.keys) / 2

	rightKeys := make([]string, len(leaf.keys)-mid)
	rightValues := make([]string, len(leaf.values)-mid)
	copy(rightKeys, leaf.keys[mid:])
	copy(rightValues, leaf.values[mid:])

	rightLeaf := &node{
		isLeaf: true,
		keys:   rightKeys,
		values: rightValues,
		prev:   leaf,
		next:   leaf.next,
	}

	if leaf.next != nil {
		leaf.next.prev = rightLeaf
	}
	leaf.next = rightLeaf

	leaf.keys = leaf.keys[:mid]
	leaf.values = leaf.values[:mid]

	midKey := rightLeaf.keys[0]

	t.insertIntoParent(leaf, midKey, rightLeaf)
}

func (t *BPlusTree) insertIntoParent(left *node, key string, right *node) {
	if left.parent == nil {
		newRoot := &node{
			isLeaf:   false,
			keys:     []string{key},
			children: []*node{left, right},
		}
		left.parent = newRoot
		right.parent = newRoot
		t.root = newRoot
		return
	}

	parent := left.parent
	idx := 0
	for idx < len(parent.keys) && parent.keys[idx] < key {
		idx++
	}

	parent.keys = append(parent.keys, "")
	for i := len(parent.keys) - 1; i > idx; i-- {
		parent.keys[i] = parent.keys[i-1]
	}
	parent.keys[idx] = key

	parent.children = append(parent.children, nil)
	for i := len(parent.children) - 1; i > idx+1; i-- {
		parent.children[i] = parent.children[i-1]
	}
	parent.children[idx+1] = right
	right.parent = parent

	if len(parent.keys) > t.maxKeys {
		t.splitInternal(parent)
	}
}

func (t *BPlusTree) splitInternal(internal *node) {
	mid := len(internal.keys) / 2
	midKey := internal.keys[mid]

	rightKeys := make([]string, len(internal.keys)-mid-1)
	rightChildren := make([]*node, len(internal.children)-mid-1)
	copy(rightKeys, internal.keys[mid+1:])
	copy(rightChildren, internal.children[mid+1:])

	rightInternal := &node{
		isLeaf:   false,
		keys:     rightKeys,
		children: rightChildren,
		parent:   internal.parent,
	}

	for _, child := range rightChildren {
		child.parent = rightInternal
	}

	internal.keys = internal.keys[:mid]
	internal.children = internal.children[:mid+1]

	t.insertIntoParent(internal, midKey, rightInternal)
}

func (t *BPlusTree) Search(key string) (string, bool) {
	if t.count == 0 {
		return "", false
	}

	leaf := t.findLeaf(key)

	for i := 0; i < len(leaf.keys); i++ {
		if leaf.keys[i] == key {
			return leaf.values[i], true
		}
	}

	return "", false
}

func (t *BPlusTree) Delete(key string) bool {
	if t.count == 0 {
		return false
	}

	leaf := t.findLeaf(key)

	idx := -1
	for i := 0; i < len(leaf.keys); i++ {
		if leaf.keys[i] == key {
			idx = i
			break
		}
	}

	if idx == -1 {
		return false
	}

	for i := idx; i < len(leaf.keys)-1; i++ {
		leaf.keys[i] = leaf.keys[i+1]
		leaf.values[i] = leaf.values[i+1]
	}
	leaf.keys = leaf.keys[:len(leaf.keys)-1]
	leaf.values = leaf.values[:len(leaf.values)-1]

	t.count--

	return true
}

func (t *BPlusTree) RangeScan(start, end string) ([]KVItem, error) {
	if start > end {
		return nil, ErrInvalidRange
	}

	var result []KVItem

	if t.count == 0 {
		return result, nil
	}

	leaf := t.findLeaf(start)

	i := 0
	for i < len(leaf.keys) && leaf.keys[i] < start {
		i++
	}

	current := leaf
	for current != nil {
		for i < len(current.keys) {
			if current.keys[i] > end {
				return result, nil
			}
			result = append(result, KVItem{
				Key:   current.keys[i],
				Value: current.values[i],
			})
			i++
		}
		current = current.next
		i = 0
	}

	return result, nil
}

type Iterator struct {
	tree   *BPlusTree
	node   *node
	index  int
	valid  bool
}

func (t *BPlusTree) NewIterator() *Iterator {
	it := &Iterator{
		tree:  t,
		valid: false,
	}

	if t.count == 0 {
		return it
	}

	current := t.root
	for !current.isLeaf {
		current = current.children[0]
	}

	it.node = current
	it.index = 0
	it.valid = true

	return it
}

func (t *BPlusTree) NewIteratorAt(key string) *Iterator {
	it := &Iterator{
		tree:  t,
		valid: false,
	}

	if t.count == 0 {
		return it
	}

	leaf := t.findLeaf(key)

	idx := 0
	for idx < len(leaf.keys) && leaf.keys[idx] < key {
		idx++
	}

	if idx >= len(leaf.keys) {
		if leaf.next != nil {
			it.node = leaf.next
			it.index = 0
			it.valid = true
		}
	} else {
		it.node = leaf
		it.index = idx
		it.valid = true
	}

	return it
}

func (it *Iterator) Valid() bool {
	return it.valid
}

func (it *Iterator) Key() (string, error) {
	if !it.valid {
		return "", ErrIteratorInvalid
	}
	return it.node.keys[it.index], nil
}

func (it *Iterator) Value() (string, error) {
	if !it.valid {
		return "", ErrIteratorInvalid
	}
	return it.node.values[it.index], nil
}

func (it *Iterator) Next() error {
	if !it.valid {
		return ErrIteratorInvalid
	}

	it.index++
	if it.index < len(it.node.keys) {
		return nil
	}

	if it.node.next != nil {
		it.node = it.node.next
		it.index = 0
		if len(it.node.keys) > 0 {
			return nil
		}
	}

	it.valid = false
	return ErrIteratorDone
}

func (it *Iterator) Prev() error {
	if !it.valid {
		return ErrIteratorInvalid
	}

	it.index--
	if it.index >= 0 {
		return nil
	}

	if it.node.prev != nil {
		it.node = it.node.prev
		it.index = len(it.node.keys) - 1
		if it.index >= 0 {
			return nil
		}
	}

	it.valid = false
	return ErrIteratorDone
}

func (it *Iterator) Delete() error {
	if !it.valid {
		return ErrIteratorInvalid
	}

	key := it.node.keys[it.index]

	leaf := it.node
	idx := it.index

	hasNext := it.index < len(leaf.keys)-1
	hasPrevNode := leaf.prev != nil && len(leaf.prev.keys) > 0

	for i := idx; i < len(leaf.keys)-1; i++ {
		leaf.keys[i] = leaf.keys[i+1]
		leaf.values[i] = leaf.values[i+1]
	}
	leaf.keys = leaf.keys[:len(leaf.keys)-1]
	leaf.values = leaf.values[:len(leaf.values)-1]

	it.tree.count--

	if len(leaf.keys) == 0 {
		if leaf.next != nil {
			leaf.next.prev = leaf.prev
		}
		if leaf.prev != nil {
			leaf.prev.next = leaf.next
		}

		if hasNext {
			it.index = idx
		} else if leaf.next != nil && len(leaf.next.keys) > 0 {
			it.node = leaf.next
			it.index = 0
		} else if hasPrevNode {
			it.node = leaf.prev
			it.index = len(leaf.prev.keys) - 1
		} else {
			it.valid = false
		}
	} else {
		if hasNext {
			it.index = idx
		} else if leaf.next != nil && len(leaf.next.keys) > 0 {
			it.node = leaf.next
			it.index = 0
		} else {
			it.valid = false
		}
	}

	_ = key
	return nil
}

func (t *BPlusTree) String() string {
	if t.count == 0 {
		return "(empty tree)"
	}

	var sb strings.Builder

	type levelNode struct {
		n     *node
		level int
	}

	queue := []levelNode{{t.root, 0}}
	currentLevel := 0

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.level != currentLevel {
			sb.WriteString("\n")
			currentLevel = item.level
		}

		if item.n.isLeaf {
			sb.WriteString(fmt.Sprintf("[%v]", item.n.keys))
		} else {
			sb.WriteString(fmt.Sprintf("(%v)", item.n.keys))
			for _, child := range item.n.children {
				queue = append(queue, levelNode{child, item.level + 1})
			}
		}
		sb.WriteString(" ")
	}

	return sb.String()
}
