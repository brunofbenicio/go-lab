package consistenthash

import (
	"fmt"
	"slices"
	"testing"
)

func keys(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("key-%d", i)
	}
	return out
}

func assignments(r *Ring, ks []string) map[string]string {
	m := make(map[string]string, len(ks))
	for _, k := range ks {
		m[k], _ = r.Get(k)
	}
	return m
}

func TestGetWithKnownPositions(t *testing.T) {
	// One replica per node and a table-driven hash give full control over
	// the ring layout: a@10, b@20, c@30.
	positions := map[string]uint32{
		"0#a": 10, "0#b": 20, "0#c": 30,
		"k1": 5, "k2": 15, "k3": 25, "k4": 30, "k5": 35,
	}
	r := New(1, WithHash(func(b []byte) uint32 { return positions[string(b)] }))
	r.Add("a", "b", "c")

	tests := []struct{ key, want string }{
		{"k1", "a"}, // first position >= 5
		{"k2", "b"},
		{"k3", "c"},
		{"k4", "c"}, // exactly on a position
		{"k5", "a"}, // past the last position: wraps to the first
	}
	for _, tt := range tests {
		if got, _ := r.Get(tt.key); got != tt.want {
			t.Errorf("Get(%s) = %s, want %s", tt.key, got, tt.want)
		}
	}
}

func TestEmptyRing(t *testing.T) {
	if node, ok := New(10).Get("x"); ok || node != "" {
		t.Errorf("Get on an empty ring = %q, %v; want \"\", false", node, ok)
	}
}

func TestAddIsIdempotentAndNodesSorted(t *testing.T) {
	r := New(10)
	r.Add("b", "a", "b")
	r.Add("a")
	if got, want := r.Nodes(), []string{"a", "b"}; !slices.Equal(got, want) {
		t.Errorf("Nodes() = %v, want %v", got, want)
	}
	if len(r.points) != 20 {
		t.Errorf("points = %d, want 20 (2 nodes x 10 replicas)", len(r.points))
	}
}

func TestMinimalDisruption(t *testing.T) {
	ks := keys(2000)
	r := New(100)
	r.Add("a", "b", "c")
	before := assignments(r, ks)

	t.Run("removal only moves the removed node's keys", func(t *testing.T) {
		r.Remove("b")
		for k, after := range assignments(r, ks) {
			if before[k] != "b" && after != before[k] {
				t.Fatalf("%s moved from %s to %s although %s was not removed", k, before[k], after, before[k])
			}
			if after == "b" {
				t.Fatalf("%s still maps to the removed node", k)
			}
		}
		r.Remove("zzz") // unknown node: no-op
	})

	t.Run("addition only moves keys to the new node", func(t *testing.T) {
		r.Add("b")
		for k, after := range assignments(r, ks) {
			if after != before[k] {
				t.Fatalf("%s changed from %s to %s after re-adding b", k, before[k], after)
			}
		}
		r.Add("d")
		moved := 0
		for k, after := range assignments(r, ks) {
			if after != before[k] {
				moved++
				if after != "d" {
					t.Fatalf("%s moved to %s, want only the new node d", k, after)
				}
			}
		}
		if moved == 0 || moved > len(ks)/2 {
			t.Errorf("moved %d of %d keys, expected a minority but not zero", moved, len(ks))
		}
	})
}

func TestDistributionIsReasonablyBalanced(t *testing.T) {
	r := New(200)
	r.Add("a", "b", "c", "d")
	ks := keys(8000)
	counts := map[string]int{}
	for _, node := range assignments(r, ks) {
		counts[node]++
	}
	for node, n := range counts {
		if share := float64(n) / float64(len(ks)); share < 0.10 || share > 0.40 {
			t.Errorf("node %s holds %.1f%% of keys, want between 10%% and 40%%", node, share*100)
		}
	}
	if len(counts) != 4 {
		t.Errorf("only %d nodes received keys, want 4", len(counts))
	}
}
