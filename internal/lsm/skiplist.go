package lsm

import (
	"math/rand"
	"time"
)

const maxLevel = 16
const p = 0.5

type SkipListNode struct {
	entry    *Entry
	forward  []*SkipListNode
	backward *SkipListNode
}

type SkipList struct {
	header *SkipListNode
	tail   *SkipListNode
	level  int
	length int
	size   int
	random *rand.Rand
}

func NewSkipList() *SkipList {
	header := &SkipListNode{
		forward: make([]*SkipListNode, maxLevel),
	}
	return &SkipList{
		header: header,
		tail:   nil,
		level:  1,
		length: 0,
		size:   0,
		random: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (sl *SkipList) randomLevel() int {
	level := 1
	for level < maxLevel && sl.random.Float64() < p {
		level++
	}
	return level
}

func (sl *SkipList) Insert(entry *Entry) {
	update := make([]*SkipListNode, maxLevel)
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].entry.Key < entry.Key {
			x = x.forward[i]
		}
		update[i] = x
	}

	if x.forward[0] != nil && x.forward[0].entry.Key == entry.Key {
		oldSize := x.forward[0].entry.Size()
		x.forward[0].entry = entry
		sl.size += entry.Size() - oldSize
		return
	}

	level := sl.randomLevel()
	if level > sl.level {
		for i := sl.level; i < level; i++ {
			update[i] = sl.header
		}
		sl.level = level
	}

	newNode := &SkipListNode{
		entry:   entry,
		forward: make([]*SkipListNode, level),
	}

	for i := 0; i < level; i++ {
		newNode.forward[i] = update[i].forward[i]
		update[i].forward[i] = newNode
	}

	newNode.backward = update[0]
	if newNode.forward[0] != nil {
		newNode.forward[0].backward = newNode
	} else {
		sl.tail = newNode
	}

	sl.length++
	sl.size += entry.Size()
}

func (sl *SkipList) Get(key string) (*Entry, bool) {
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].entry.Key < key {
			x = x.forward[i]
		}
	}
	x = x.forward[0]
	if x != nil && x.entry.Key == key {
		return x.entry, true
	}
	return nil, false
}

func (sl *SkipList) Delete(key string) (*Entry, bool) {
	update := make([]*SkipListNode, maxLevel)
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].entry.Key < key {
			x = x.forward[i]
		}
		update[i] = x
	}

	x = x.forward[0]
	if x == nil || x.entry.Key != key {
		return nil, false
	}

	for i := 0; i < sl.level; i++ {
		if update[i].forward[i] != x {
			break
		}
		update[i].forward[i] = x.forward[i]
	}

	if x.forward[0] != nil {
		x.forward[0].backward = x.backward
	} else {
		sl.tail = x.backward
	}

	for sl.level > 1 && sl.header.forward[sl.level-1] == nil {
		sl.level--
	}

	sl.length--
	sl.size -= x.entry.Size()

	return x.entry, true
}

func (sl *SkipList) Len() int {
	return sl.length
}

func (sl *SkipList) Size() int {
	return sl.size
}

func (sl *SkipList) Iterator() *SkipListIterator {
	return &SkipListIterator{
		sl:      sl,
		current: sl.header.forward[0],
	}
}

func (sl *SkipList) Range(start, end string) []*Entry {
	var result []*Entry
	iter := sl.Iterator()
	for iter.Next() {
		key := iter.Entry().Key
		if key > end {
			break
		}
		if key >= start {
			result = append(result, iter.Entry())
		}
	}
	return result
}

func (sl *SkipList) AllEntries() []*Entry {
	result := make([]*Entry, 0, sl.length)
	iter := sl.Iterator()
	for iter.Next() {
		result = append(result, iter.Entry())
	}
	return result
}

type SkipListIterator struct {
	sl      *SkipList
	current *SkipListNode
	started bool
}

func (it *SkipListIterator) Next() bool {
	if !it.started {
		it.started = true
		return it.current != nil
	}
	if it.current == nil {
		return false
	}
	it.current = it.current.forward[0]
	return it.current != nil
}

func (it *SkipListIterator) Entry() *Entry {
	if it.current == nil {
		return nil
	}
	return it.current.entry
}

func (it *SkipListIterator) HasNext() bool {
	if !it.started {
		return it.current != nil
	}
	return it.current != nil && it.current.forward[0] != nil
}

func (it *SkipListIterator) Seek(key string) {
	x := it.sl.header
	for i := it.sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].entry.Key < key {
			x = x.forward[i]
		}
	}
	it.current = x.forward[0]
	it.started = false
}
