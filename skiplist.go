// Package skiplist defines a probabilistic ordered map backed by a skip list.
package skiplist

import (
	"cmp"
	"fmt"
	"iter"
	"math/rand"
	"strings"
	"time"
)

// SkipList is an ordered map: keys are kept sorted, with O(log n) expected
// Search/Insert/Delete. Iteration is ascending by key.
type SkipList[K cmp.Ordered, V any] interface {
	// Get returns the value stored for key and whether it was present.
	Get(key K) (value V, ok bool)

	// Set inserts or overwrites the value for key.
	Set(key K, value V)

	// Delete removes key, reporting whether it had been present.
	Delete(key K) (deleted bool)

	// Len returns the number of distinct keys stored.
	Len() int

	// All returns an iterator over key/value pairs in ascending key order.
	All() iter.Seq2[K, V]

	// Min returns the smallest key/value; ok is false if the list is empty.
	Min() (key K, value V, ok bool)

	// Max returns the largest key/value; ok is false if the list is empty.
	Max() (key K, value V, ok bool)
}

var (
	_ SkipList[int, string] = (*List[int, string])(nil)
	_ fmt.Stringer          = (*List[int, string])(nil)
)

// defaultMaxLevel caps the tower height. With the default probability, this
// comfortably supports ~4^18 ≈ 6.8e10 elements before the expected-O(log n)
// guarantee degrades.
const defaultMaxLevel = 18

// defaultProbability is the promotion probability for randomTopLevel: each
// higher level is reached with probability 1/4, trading a bit of search speed
// for fewer pointers.
const defaultProbability = 0.25

type config struct {
	maxLevel    int
	probability float64
	randFloat   func() float64
}

// Option configures a List.
type Option func(*config)

// WithMaxLevel sets the maximum tower height.
func WithMaxLevel(maxLevel int) Option {
	return func(cfg *config) {
		if maxLevel <= 0 {
			panic("skiplist: max level must be positive")
		}
		cfg.maxLevel = maxLevel
	}
}

// WithProbability sets the promotion probability for each higher level.
func WithProbability(probability float64) Option {
	return func(cfg *config) {
		if probability < 0 || probability >= 1 {
			panic("skiplist: probability must be in [0, 1)")
		}
		cfg.probability = probability
	}
}

// WithRandFloat sets the random source used for tower promotion decisions.
// randFloat must behave like rand.Float64, returning values in [0, 1).
func WithRandFloat(randFloat func() float64) Option {
	return func(cfg *config) {
		if randFloat == nil {
			randFloat = newRandFloat()
		}
		cfg.randFloat = randFloat
	}
}

// WithRand sets the random source used for tower promotion decisions.
func WithRand(rng *rand.Rand) Option {
	return func(cfg *config) {
		if rng == nil {
			cfg.randFloat = newRandFloat()
			return
		}
		cfg.randFloat = rng.Float64
	}
}

func newRandFloat() func() float64 {
	return rand.New(rand.NewSource(time.Now().UnixNano())).Float64
}

// node is one entry in the list. forward[i] is the next node at level i.
// A node participates in levels 0..len(forward)-1.
type node[K cmp.Ordered, V any] struct {
	key     K
	value   V
	forward []*node[K, V]
}

// List is the concrete skip list. The zero value is ready to use; New should be
// used when passing options. The head sentinel carries no real key/value.
//
// A List is not safe for concurrent use by multiple goroutines. If a List is
// shared across goroutines, callers must serialize access externally.
type List[K cmp.Ordered, V any] struct {
	head        *node[K, V]
	tail        *node[K, V]
	topLevel    int // highest level currently in use (0-based), i.e. len-1 of tallest tower
	length      int
	maxLevel    int
	probability float64
	randFloat   func() float64
}

// New returns an empty skip list ready to use.
func New[K cmp.Ordered, V any](opts ...Option) *List[K, V] {
	cfg := config{
		maxLevel:    defaultMaxLevel,
		probability: defaultProbability,
		randFloat:   newRandFloat(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return &List[K, V]{
		head:        &node[K, V]{forward: make([]*node[K, V], cfg.maxLevel)},
		maxLevel:    cfg.maxLevel,
		probability: cfg.probability,
		randFloat:   cfg.randFloat,
	}
}

func (l *List[K, V]) init() {
	if l.head != nil {
		return
	}
	maxLevel := l.maxLevel
	if maxLevel <= 0 {
		maxLevel = defaultMaxLevel
	}
	probability := l.probability
	if probability == 0 {
		probability = defaultProbability
	}
	randFloat := l.randFloat
	if randFloat == nil {
		randFloat = newRandFloat()
	}

	l.head = &node[K, V]{forward: make([]*node[K, V], maxLevel)}
	l.maxLevel = maxLevel
	l.probability = probability
	l.randFloat = randFloat
}

// randomTopLevel returns a zero-based top level in [0, maxLevel-1],
// geometrically distributed with parameter p.
func (l *List[K, V]) randomTopLevel() int {
	if l.probability == 0 {
		return 0
	}
	topLevel := 0
	for topLevel < l.maxLevel-1 && l.randFloat() < l.probability {
		topLevel++
	}
	return topLevel
}

func (l *List[K, V]) updateScratch(stack *[defaultMaxLevel]*node[K, V]) []*node[K, V] {
	if l.maxLevel <= len(stack) {
		return stack[:l.maxLevel]
	}
	return make([]*node[K, V], l.maxLevel)
}

// Get returns the value stored for key and whether it was present.
func (l *List[K, V]) Get(key K) (value V, ok bool) {
	if l.length == 0 {
		return value, false
	}

	current := l.head
	for level := l.topLevel; level >= 0; level-- {
		for current.forward[level] != nil && cmp.Less(current.forward[level].key, key) {
			current = current.forward[level]
		}
	}
	candidate := current.forward[0]
	if candidate != nil && cmp.Compare(candidate.key, key) == 0 {
		return candidate.value, true
	}
	return value, false
}

// Set inserts or overwrites the value for key.
func (l *List[K, V]) Set(key K, value V) {
	l.init()

	if l.length == 0 {
		newTopLevel := l.randomTopLevel()
		newNode := &node[K, V]{key: key, value: value, forward: make([]*node[K, V], newTopLevel+1)}
		for level := range newTopLevel + 1 {
			l.head.forward[level] = newNode
		}
		l.tail = newNode
		l.topLevel = newTopLevel
		l.length = 1
		return
	}

	var updateStack [defaultMaxLevel]*node[K, V]
	update := l.updateScratch(&updateStack)
	current := l.head
	for level := l.topLevel; level >= 0; level-- {
		for current.forward[level] != nil && cmp.Less(current.forward[level].key, key) {
			current = current.forward[level]
		}
		update[level] = current
	}
	candidate := current.forward[0]
	if candidate != nil && cmp.Compare(candidate.key, key) == 0 {
		candidate.value = value
		return
	}
	newTopLevel := l.randomTopLevel()
	if newTopLevel > l.topLevel {
		for level := l.topLevel + 1; level <= newTopLevel; level++ {
			update[level] = l.head
		}
		l.topLevel = newTopLevel
	}
	newNode := &node[K, V]{key: key, value: value, forward: make([]*node[K, V], newTopLevel+1)}
	for level := range newTopLevel + 1 {
		newNode.forward[level] = update[level].forward[level]
		update[level].forward[level] = newNode
	}
	if newNode.forward[0] == nil {
		l.tail = newNode
	}
	l.length++
}

// Delete removes key, reporting whether it had been present.
func (l *List[K, V]) Delete(key K) (deleted bool) {
	if l.length == 0 {
		return false
	}

	var updateStack [defaultMaxLevel]*node[K, V]
	update := l.updateScratch(&updateStack)
	current := l.head

	for level := l.topLevel; level >= 0; level-- {
		for current.forward[level] != nil && cmp.Less(current.forward[level].key, key) {
			current = current.forward[level]
		}
		update[level] = current
	}
	target := current.forward[0]
	if target == nil || cmp.Compare(target.key, key) != 0 {
		return false
	}

	if target == l.tail {
		l.tail = update[0]
		if l.tail == l.head {
			l.tail = nil
		}
	}
	for level := 0; level <= l.topLevel; level++ {
		if update[level].forward[level] == target {
			update[level].forward[level] = target.forward[level]
		}
	}

	for l.topLevel > 0 && l.head.forward[l.topLevel] == nil {
		l.topLevel--
	}
	l.length--

	return true
}

// Len returns the number of distinct keys stored.
func (l *List[K, V]) Len() int {
	return l.length
}

// All returns an iterator over key/value pairs in ascending key order.
func (l *List[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if l.head == nil {
			return
		}
		for current := l.head.forward[0]; current != nil; current = current.forward[0] {
			if !yield(current.key, current.value) {
				return
			}
		}
	}
}

// Range iterates keys in ascending order until fn returns false.
func (l *List[K, V]) Range(fn func(key K, value V) bool) {
	l.All()(fn)
}

// String returns a deterministic representation of the list in ascending key
// order. It is intended for debugging and test output.
func (l *List[K, V]) String() string {
	var b strings.Builder
	b.WriteString("skiplist[")
	i := 0
	for key, value := range l.All() {
		if i > 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%v:%v", key, value)
		i++
	}
	b.WriteString("]")
	return b.String()
}

// Min returns the smallest key/value; ok is false if the list is empty.
func (l *List[K, V]) Min() (key K, value V, ok bool) {
	if l.length == 0 {
		return key, value, false
	}
	first := l.head.forward[0]
	return first.key, first.value, true
}

// Max returns the largest key/value; ok is false if the list is empty.
func (l *List[K, V]) Max() (key K, value V, ok bool) {
	if l.tail == nil {
		return key, value, false
	}
	return l.tail.key, l.tail.value, true
}
