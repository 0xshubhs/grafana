package buffer

import (
	"sync/atomic"
)

// Sample represents a single metric sample with timestamp and value
type Sample struct {
	Ts  int64
	Val float64
}

// Ring is a lock-free ring buffer for metric samples
// Optimized for single-writer, multiple-reader access pattern
type Ring struct {
	data []Sample
	idx  atomic.Uint64
	size uint64
}

// NewRing creates a new ring buffer with the specified size
func NewRing(size int) *Ring {
	return &Ring{
		data: make([]Sample, size),
		size: uint64(size),
	}
}

// Push adds a sample to the ring buffer (lock-free)
// Safe for single writer only
func (r *Ring) Push(s Sample) {
	i := r.idx.Add(1) - 1
	r.data[i%r.size] = s
}

// Snapshot returns a copy of all samples in order (oldest to newest)
// Safe for concurrent reads
func (r *Ring) Snapshot() []Sample {
	currentIdx := r.idx.Load()
	result := make([]Sample, 0, r.size)

	var start uint64
	if currentIdx >= r.size {
		start = currentIdx - r.size
	}

	for i := start; i < currentIdx; i++ {
		result = append(result, r.data[i%r.size])
	}

	return result
}

// SnapshotLast returns the last n samples
func (r *Ring) SnapshotLast(n int) []Sample {
	currentIdx := r.idx.Load()
	count := uint64(n)
	if count > r.size {
		count = r.size
	}
	if count > currentIdx {
		count = currentIdx
	}

	result := make([]Sample, 0, count)
	start := currentIdx - count

	for i := start; i < currentIdx; i++ {
		result = append(result, r.data[i%r.size])
	}

	return result
}

// Latest returns the most recent sample
func (r *Ring) Latest() (Sample, bool) {
	currentIdx := r.idx.Load()
	if currentIdx == 0 {
		return Sample{}, false
	}
	return r.data[(currentIdx-1)%r.size], true
}

// Count returns the total number of samples written
func (r *Ring) Count() uint64 {
	return r.idx.Load()
}

// Len returns the current number of valid samples in the buffer
func (r *Ring) Len() int {
	count := r.idx.Load()
	if count > r.size {
		return int(r.size)
	}
	return int(count)
}
