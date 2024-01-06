// Package kway impements k-way merge algorithms for range functions.
package kway

import (
	"cmp"
	"iter"
)

const (
	bufferSize = 128
)

// Merge merges multiple sequences into one. The sequences must produce ordered
// values.
func Merge[V cmp.Ordered](seqs ...iter.Seq[V]) iter.Seq[V] {
	return MergeFunc(cmp.Compare[V], seqs...)
}

// MergeFunc merges multiple sequences into one using the given comparison
// function to determine the order of values. The sequences must be ordered
// by the same comparison function.
func MergeFunc[V any](cmp func(V, V) int, seqs ...iter.Seq[V]) iter.Seq[V] {
	switch len(seqs) {
	case 0:
		return func(func(V) bool) {}
	case 1:
		return seqs[0]
	case 2:
		seq0 := bufferedFunc(bufferSize, seqs[0])
		seq1 := bufferedFunc(bufferSize, seqs[1])
		return debuffer(merge2(cmp, seq0, seq1))
	default:
		bufferedSeqs := make([]iter.Seq[[]V], len(seqs))
		for i, seq := range seqs {
			bufferedSeqs[i] = bufferedFunc(bufferSize, seq)
		}
		return debuffer(merge(cmp, bufferedSeqs))
	}
}

func MergeSlice[V cmp.Ordered](seqs ...iter.Seq[[]V]) iter.Seq[[]V] {
	return MergeSliceFunc(cmp.Compare[V], seqs...)
}

func MergeSliceFunc[V any](cmp func(V, V) int, seqs ...iter.Seq[[]V]) iter.Seq[[]V] {
	switch len(seqs) {
	case 0:
		return func(func([]V) bool) {}
	case 1:
		return seqs[0]
	case 2:
		return merge2(cmp, seqs[0], seqs[1])
	default:
		return merge(cmp, seqs)
	}
}

//go:noinline
func debuffer[V any](seq iter.Seq[[]V]) iter.Seq[V] {
	return func(yield func(V) bool) {
		seq(func(values []V) bool {
			for _, value := range values {
				if !yield(value) {
					return false
				}
			}
			return true
		})
	}
}

//go:noinline
func merge2[V any](cmp func(V, V) int, seq0, seq1 iter.Seq[[]V]) iter.Seq[[]V] {
	return func(yield func([]V) bool) {
		next0, stop0 := iter.Pull(seq0)
		defer stop0()

		next1, stop1 := iter.Pull(seq1)
		defer stop1()

		values0, ok0 := next0()
		values1, ok1 := next1()
		buffer := make([]V, bufferSize)
		offset := 0

		for ok0 && ok1 {
			i0 := 0
			i1 := 0

			for i0 < len(values0) && i1 < len(values1) {
				v0 := values0[i0]
				v1 := values1[i1]

				if (offset + 1) >= len(buffer) {
					if !yield(buffer[:offset]) {
						return
					}
					offset = 0
				}

				diff := cmp(v0, v1)
				switch {
				case diff < 0:
					buffer[offset] = v0
					offset++
					i0++
				case diff > 0:
					buffer[offset] = v1
					offset++
					i1++
				default:
					buffer[offset+0] = v0
					buffer[offset+1] = v1
					offset += 2
					i0++
					i1++
				}
			}

			if i0 == len(values0) {
				i0 = 0
				values0, ok0 = next0()
			}

			if i1 == len(values1) {
				i1 = 0
				values1, ok1 = next1()
			}
		}

		if offset > 0 {
			if !yield(buffer[:offset]) {
				return
			}
		}

		flush(yield, next0, values0, ok0)
		flush(yield, next1, values1, ok1)
	}
}

func flush[V any](yield func([]V) bool, next func() ([]V, bool), values []V, ok bool) {
	for ok && yield(values) {
		values, ok = next()
	}
}

//go:noinline
func merge[V any](cmp func(V, V) int, seqs []iter.Seq[[]V]) iter.Seq[[]V] {
	return func(yield func([]V) bool) {
		tree := makeTree(seqs...)
		defer tree.stop()

		buffer := make([]V, bufferSize)
		offset := 0

		for {
			v, ok := tree.next(cmp)
			if !ok {
				break
			}
			buffer[offset] = v
			offset++
			if offset == len(buffer) {
				if !yield(buffer) {
					return
				}
				offset = 0
			}
		}

		if offset > 0 {
			yield(buffer[:offset])
		}
	}
}
