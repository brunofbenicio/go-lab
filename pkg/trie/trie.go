// Package trie implements a generic prefix tree keyed by strings. It indexes
// keys by rune, so multi-byte text works correctly, and supports prefix
// queries such as autocomplete.
//
// A Trie is not safe for concurrent use.
package trie

import "sort"

type node[V any] struct {
	children map[rune]*node[V]
	value    V
	terminal bool // a key ends here
}

// Trie maps string keys to values of type V.
type Trie[V any] struct {
	root *node[V]
	size int
}

// New returns an empty trie.
func New[V any]() *Trie[V] { return &Trie[V]{root: &node[V]{}} }

// Put stores value under key, replacing any previous value.
func (t *Trie[V]) Put(key string, value V) {
	n := t.root
	for _, r := range key {
		child, ok := n.children[r]
		if !ok {
			if n.children == nil {
				n.children = make(map[rune]*node[V])
			}
			child = &node[V]{}
			n.children[r] = child
		}
		n = child
	}
	if !n.terminal {
		t.size++
	}
	n.value, n.terminal = value, true
}

// Get returns the value stored under key.
func (t *Trie[V]) Get(key string) (V, bool) {
	if n := t.find(key); n != nil && n.terminal {
		return n.value, true
	}
	var zero V
	return zero, false
}

// HasPrefix reports whether any stored key starts with prefix.
func (t *Trie[V]) HasPrefix(prefix string) bool { return t.find(prefix) != nil }

// Len returns the number of stored keys.
func (t *Trie[V]) Len() int { return t.size }

// KeysWithPrefix returns, in lexicographic order, every stored key that starts
// with prefix. An empty prefix returns all keys.
func (t *Trie[V]) KeysWithPrefix(prefix string) []string {
	var keys []string
	if n := t.find(prefix); n != nil {
		collect(n, []rune(prefix), &keys)
	}
	sort.Strings(keys)
	return keys
}

// Delete removes key and reports whether it was present. Branches left
// without keys are pruned.
func (t *Trie[V]) Delete(key string) bool {
	runes := []rune(key)
	var del func(n *node[V], i int) (deleted, prune bool)
	del = func(n *node[V], i int) (bool, bool) {
		if i == len(runes) {
			if !n.terminal {
				return false, false
			}
			var zero V
			n.value, n.terminal = zero, false
			return true, len(n.children) == 0
		}
		child, ok := n.children[runes[i]]
		if !ok {
			return false, false
		}
		deleted, prune := del(child, i+1)
		if prune {
			delete(n.children, runes[i])
		}
		return deleted, deleted && !n.terminal && len(n.children) == 0
	}
	deleted, _ := del(t.root, 0)
	if deleted {
		t.size--
	}
	return deleted
}

// find returns the node reached by following key, or nil.
func (t *Trie[V]) find(key string) *node[V] {
	n := t.root
	for _, r := range key {
		if n = n.children[r]; n == nil {
			return nil
		}
	}
	return n
}

func collect[V any](n *node[V], path []rune, out *[]string) {
	if n.terminal {
		*out = append(*out, string(path))
	}
	for r, child := range n.children {
		collect(child, append(path[:len(path):len(path)], r), out)
	}
}
