package slice

import (
	"errors"
	"slices"
	"sort"
	"sync"
)

var ErrSliceEmpty = errors.New("slice is empty")

type Slice[T comparable] struct {
	mu    sync.Mutex
	items []T
}

// ForEach loops all items.
func (s *Slice[T]) ForEach(fn func(T)) {
	for i := range s.items {
		fn(s.items[i])
	}
}

// ForEachMut applies the given function to each element of the slice, allowing
// the function to mutate the elements.
func (s *Slice[T]) ForEachMut(fn func(*T)) {
	for i := range s.items {
		fn(&s.items[i])
	}
}

// ForEachErr loops all items and returns a err.
func (s *Slice[T]) ForEachErr(fn func(T) error) error {
	for i := range s.items {
		if err := fn(s.items[i]); err != nil {
			return err
		}
	}

	return nil
}

// ForEachMutErr applies the given function to each element of the slice, allowing
// the function to mutate the elements.
func (s *Slice[T]) ForEachMutErr(fn func(*T) error) error {
	for i := range s.items {
		if err := fn(&s.items[i]); err != nil {
			return err
		}
	}

	return nil
}

// ForEachIdx loop items with index.
func (s *Slice[T]) ForEachIdx(fn func(int, T)) {
	for i, ele := range s.items {
		fn(i, ele)
	}
}

// ForEachIdxErr loop items with index and returns a error.
func (s *Slice[T]) ForEachIdxErr(fn func(int, T) error) error {
	for i, ele := range s.items {
		if err := fn(i, ele); err != nil {
			return err
		}
	}

	return nil
}

// Filter creates a new Slice containing only the elements that satisfy the
// given predicate.
func (s *Slice[T]) Filter(predicate func(T) bool) *Slice[T] {
	filtered := make([]T, 0, len(s.items))
	for _, item := range s.items {
		if predicate(item) {
			filtered = append(filtered, item)
		}
	}

	return &Slice[T]{items: filtered}
}

// FilterInPlace filters the items in place with a callback.
func (s *Slice[T]) FilterInPlace(fn func(*T) bool) {
	filtered := s.items[:0]
	for i := range s.items {
		if fn(&s.items[i]) {
			filtered = append(filtered, s.items[i])
		}
	}
	s.items = filtered
}

// Includes checks if the item is in the slice.
func (s *Slice[T]) Includes(target *T) bool {
	return slices.Contains(s.items, *target)
}

// Any checks if any item matches the predicate.
func (s *Slice[T]) Any(predicate func(T) bool) bool {
	return slices.ContainsFunc(s.items, predicate)
}

// Head returns the first n items.
func (s *Slice[T]) Head(n int) {
	l := s.Len()
	if n <= 0 || n > l || l == 0 {
		return
	}
	s.items = (s.items)[:n]
}

// Tail returns the last n items.
func (s *Slice[T]) Tail(n int) {
	l := s.Len()
	if n <= 0 || n > l || l == 0 {
		return
	}
	tail := l - n
	s.items = (s.items)[tail:]
}

// Len returns the length of the items.
func (s *Slice[T]) Len() int {
	return len(s.items)
}

// Push adds a single item to the items.
func (s *Slice[T]) Push(item *T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, *item)
}

// Append adds multiple items to the items.
func (s *Slice[T]) Append(elements ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ele := range elements {
		if !slices.Contains(s.items, ele) {
			s.items = append(s.items, ele)
		}
	}
}

// Set sets the items.
func (s *Slice[T]) Set(items *[]T) {
	s.items = *items
}

// Item returns a single item by index.
func (s *Slice[T]) Item(i int) T {
	return (s.items)[i]
}

// Items returns all items in the slice.
func (s *Slice[T]) Items() *[]T {
	return &s.items
}

// Index returns the index of the item.
func (s *Slice[T]) Index(item T) int {
	return slices.Index(s.items, item)
}

// Del removes an items from the slice.
func (s *Slice[T]) Del(item T) {
	idx := s.Index(item)
	if idx != -1 {
		s.items = slices.Delete(s.items, idx, idx+1)
	}
}

// Clean removes all items from the slice.
func (s *Slice[T]) Clean() {
	s.items = make([]T, 0)
}

// Empty returns true if the slice is empty.
func (s *Slice[T]) Empty() bool {
	return len(s.items) == 0
}

// Sort sorts the items in the slice based on the provided less function.
func (s *Slice[T]) Sort(less func(a, b T) bool) {
	sort.Slice(s.items, func(i, j int) bool {
		return less((s.items)[i], (s.items)[j])
	})
}

// New creates a new slice of type T.
func New[T comparable](items ...T) *Slice[T] {
	return &Slice[T]{items: items}
}
