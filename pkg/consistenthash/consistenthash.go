// Package consistenthash implements a consistent hash ring with virtual
// nodes. Keys are mapped to nodes so that adding or removing a node moves only
// a small fraction of the keys, which makes it a good fit for sharded caches
// and load balancing.
package consistenthash

import (
	"hash/crc32"
	"slices"
	"sort"
	"strconv"
	"sync"
)

// defaultReplicas is used when New receives a non-positive replica count.
const defaultReplicas = 100

// Ring is a consistent hash ring. It is safe for concurrent use.
type Ring struct {
	mu       sync.RWMutex
	replicas int
	hash     func([]byte) uint32
	points   []uint32          // sorted positions of all virtual nodes
	owners   map[uint32]string // position -> node
	nodes    map[string]struct{}
}

// Option customizes a Ring.
type Option func(*Ring)

// WithHash replaces the hash function (default: CRC-32 IEEE).
func WithHash(fn func([]byte) uint32) Option { return func(r *Ring) { r.hash = fn } }

// New creates an empty ring where each node is placed at replicas virtual
// positions. More replicas give a more even distribution. A replicas value
// < 1 selects the default of 100.
func New(replicas int, opts ...Option) *Ring {
	if replicas < 1 {
		replicas = defaultReplicas
	}
	r := &Ring{
		replicas: replicas,
		hash:     crc32.ChecksumIEEE,
		owners:   make(map[uint32]string),
		nodes:    make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Add inserts nodes into the ring. Nodes already present are ignored. If two
// virtual nodes collide on the same position, the first one added wins.
func (r *Ring) Add(nodes ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, node := range nodes {
		if _, ok := r.nodes[node]; ok {
			continue
		}
		r.nodes[node] = struct{}{}
		for i := 0; i < r.replicas; i++ {
			pos := r.hash([]byte(strconv.Itoa(i) + "#" + node))
			if _, taken := r.owners[pos]; taken {
				continue
			}
			r.owners[pos] = node
			r.points = append(r.points, pos)
		}
	}
	slices.Sort(r.points)
}

// Remove deletes node from the ring. Only the keys that belonged to it move.
func (r *Ring) Remove(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.nodes[node]; !ok {
		return
	}
	delete(r.nodes, node)
	kept := r.points[:0]
	for _, pos := range r.points {
		if r.owners[pos] == node {
			delete(r.owners, pos)
			continue
		}
		kept = append(kept, pos)
	}
	r.points = kept
}

// Get returns the node responsible for key. The second result is false if the
// ring has no nodes.
func (r *Ring) Get(key string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.points) == 0 {
		return "", false
	}
	h := r.hash([]byte(key))
	i := sort.Search(len(r.points), func(i int) bool { return r.points[i] >= h })
	if i == len(r.points) {
		i = 0 // wrap around the ring
	}
	return r.owners[r.points[i]], true
}

// Nodes returns the nodes in the ring, sorted by name.
func (r *Ring) Nodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.nodes))
	for n := range r.nodes {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}
