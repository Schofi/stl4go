package stl4go

import (
	"math/rand"
	"time"
)

const (
	skipListMaxLevel = 40
)

// SkipList is a probabilistic data structure that seem likely to supplant balanced trees as the
// implementation method of choice for many applications. Skip list algorithms have the same
// asymptotic expected time bounds as balanced trees and are simpler, faster and use less space.
//
// See https://en.wikipedia.org/wiki/Skip_list for more details.
type SkipList[K Ordered, V any] struct {
	keyCmp CompareFn[K]
	level  int // Current level, may increase dynamically during insertion
	len    int // Total elements numner in the skiplist.
	head   skipListNode[K, V]
	// This cache is used to save the previous nodes when modifying the skip list to avoid
	// allocating memory each time it is called.
	prevsCache []*skipListNode[K, V] // Cache to avoid memory allocation.
	rander     *rand.Rand
}

type skipListNode[K Ordered, V any] struct {
	key   K
	value V
	level int
	next  []*skipListNode[K, V]
}

// NewSkipList create a new Skiplist.
func NewSkipList[K Ordered, V any]() *SkipList[K, V] {
	l := &SkipList[K, V]{
		level:  1,
		keyCmp: OrderedCompare[K],
		// #nosec G404 -- This is not a security condition
		rander:     rand.New(rand.NewSource(time.Now().Unix())),
		prevsCache: make([]*skipListNode[K, V], skipListMaxLevel),
	}
	l.head.next = make([]*skipListNode[K, V], skipListMaxLevel)
	return l
}

// NewSkipListFromMap create a new Skiplist from a map.
func NewSkipListFromMap[K Ordered, V any](m map[K]V) *SkipList[K, V] {
	sl := NewSkipList[K, V]()
	for k, v := range m {
		sl.Insert(k, v)
	}
	return sl
}

func (sl *SkipList[K, V]) IsEmpty() bool {
	return sl.len == 0
}

func (sl *SkipList[K, V]) Len() int {
	return sl.len
}

func (sl *SkipList[K, V]) Clear() {
	for i := range sl.head.next {
		sl.head.next[i] = nil
	}
	sl.level = 1
	sl.len = 0
}

// Insert inserts a key-value pair into the skiplist
func (sl *SkipList[K, V]) Insert(key K, value V) {
	eq, prevs := sl.findInsertPoint(key)

	if eq != nil {
		// Already exist, update the value
		eq.value = value
		return
	}

	level := sl.randomLevel()

	e := &skipListNode[K, V]{
		key:   key,
		value: value,
		level: level,
		next:  make([]*skipListNode[K, V], level),
	}

	for i := 0; i < Min(level, sl.level); i++ {
		e.next[i] = prevs[i].next[i]
		prevs[i].next[i] = e
	}

	if level > sl.level {
		for i := sl.level; i < level; i++ {
			sl.head.next[i] = e
		}
		sl.level = level
	}

	sl.len++
}

// Find returns the value associated with the passed key if the key is in the skiplist, otherwise
// returns nil.
func (sl *SkipList[K, V]) Find(key K) *V {
	node := sl.findNode(key)
	if node != nil {
		return &node.value
	}
	return nil
}

func (sl *SkipList[K, V]) Has(key K) bool {
	return sl.findNode(key) != nil
}

// Remove removes the key-value pair associated with the passed key and returns true if the key is
// in the skiplist, otherwise returns false.
func (sl *SkipList[K, V]) Remove(key K) bool {
	node, prevs := sl.findRemovePoint(key)
	if node == nil {
		return false
	}
	for i, v := range node.next {
		prevs[i].next[i] = v
	}
	for sl.level > 2 && sl.head.next[sl.level-1] == nil {
		sl.level--
	}
	sl.len--
	return true
}

func (sl *SkipList[K, V]) ForEach(op func(K, *V)) {
	for e := sl.head.next[0]; e != nil; e = e.next[0] {
		op(e.key, &e.value)
	}
}

func (sl *SkipList[K, V]) ForEachIf(op func(K, *V) bool) {
	for e := sl.head.next[0]; e != nil; e = e.next[0] {
		if !op(e.key, &e.value) {
			return
		}
	}
}

func (sl *SkipList[K, V]) randomLevel() int {
	total := uint64(1)<<uint64(skipListMaxLevel) - 1 // 2^n-1
	k := sl.rander.Uint64() % total
	levelN := uint64(1) << (uint64(skipListMaxLevel) - 1)

	level := 1
	for total -= levelN; total > k; level++ {
		levelN >>= 1
		total -= levelN
		// Since levels are randomly generated, most should be less than log2(s.len).
		// Then make a limit according to sl.len to avoid unexpectedly large value.
		if level > 2 && 1<<(level-2) > sl.len {
			break
		}
	}
	return level
}

func (sl *SkipList[K, V]) findNode(key K) *skipListNode[K, V] {
	var pre = &sl.head
	for i := sl.level - 1; i >= 0; i-- {
		cur := pre.next[i]
		for ; cur != nil; cur = cur.next[i] {
			cmpRet := sl.keyCmp(cur.key, key)
			if cmpRet == 0 {
				return cur
			}
			if cmpRet > 0 {
				break
			}
			pre = cur
		}
	}
	return nil
}

// findInsertPoint returns (*node, nil) to the existed node if the key exists,
// or (nil, []*node) to the previous nodes if the key doesn't exist
func (sl *SkipList[K, V]) findInsertPoint(key K) (*skipListNode[K, V], []*skipListNode[K, V]) {
	prevs := sl.prevsCache[0:sl.level]
	prev := &sl.head
	for i := sl.level - 1; i >= 0; i-- {
		if sl.head.next[i] != nil {
			for next := prev.next[i]; next != nil; next = next.next[i] {
				r := sl.keyCmp(next.key, key)
				if r == 0 {
					return next, nil
				}
				if r > 0 {
					break
				}
				prev = next
			}
		}
		prevs[i] = prev
	}
	return nil, prevs
}

func (sl *SkipList[K, V]) findRemovePoint(key K) (*skipListNode[K, V], []*skipListNode[K, V]) {
	prevs := sl.findPrevNodes(key)
	node := prevs[0].next[0]
	if node == nil {
		return nil, nil
	}
	if node != nil && sl.keyCmp(node.key, key) != 0 {
		return nil, nil
	}
	return node, prevs
}

func (sl *SkipList[K, V]) findPrevNodes(key K) []*skipListNode[K, V] {
	prevs := sl.prevsCache[0:sl.level]
	prev := &sl.head
	for i := sl.level - 1; i >= 0; i-- {
		if sl.head.next[i] != nil {
			for next := prev.next[i]; next != nil; next = next.next[i] {
				if sl.keyCmp(next.key, key) >= 0 {
					break
				}
				prev = next
			}
		}
		prevs[i] = prev
	}
	return prevs
}