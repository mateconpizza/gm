package slice

import (
	"golang.org/x/exp/slices"
)

type Slice[T comparable] struct {
	items *[]T
}

// ForEach loops all items.
func (s *Slice[T]) ForEach(fn func(T)) {
	for _, ele := range *s.items {
		fn(ele)
	}
}

// ForEachErr loops all items and returns a err.
func (s *Slice[T]) ForEachErr(fn func(T) error) error {
	for _, ele := range *s.items {
		if err := fn(ele); err != nil {
			return err
		}
	}

	return nil
}

// ForEachIdx loop items all items with index.
func (s *Slice[T]) ForEachIdx(fn func(int, T)) {
	for i, ele := range *s.items {
		fn(i, ele)
	}
}

// ForEachErrIdx loop items all items with index and returns a err.
func (s *Slice[T]) ForEachErrIdx(fn func(int, T) error) error {
	for i, ele := range *s.items {
		if err := fn(i, ele); err != nil {
			return err
		}
	}

	return nil
}

// Filter filters the items with a callback.
func (s *Slice[T]) Filter(fn func(T) bool) {
	slice := New[T]()
	for _, ele := range *s.items {
		t := ele
		if fn(t) {
			slice.Append(&t)
		}
	}
	*s.items = *slice.items
}

// Has checks if the item is in the slice.
func (s *Slice[T]) Has(fn func(T) bool) bool {
	for _, ele := range *s.items {
		if fn(ele) {
			return true
		}
	}

	return false
}

// Head returns the first n items.
func (s *Slice[T]) Head(n int) {
	l := s.Len()
	if n <= 0 || n > l || l == 0 {
		return
	}
	*s.items = (*s.items)[:n]
}

// Tail returns the last n items.
func (s *Slice[T]) Tail(n int) {
	l := s.Len()
	if n <= 0 || n > l || l == 0 {
		return
	}
	tail := l - n
	*s.items = (*s.items)[tail:]
}

// Len returns the length of the items.
func (s *Slice[T]) Len() int {
	return len(*s.items)
}

// Append adds a single item to the items.
func (s *Slice[T]) Append(ele *T) {
	if slices.Contains(*s.items, *ele) {
		return
	}

	*s.items = append(*s.items, *ele)
}

// Set sets the items.
func (s *Slice[T]) Set(items *[]T) {
	*s.items = *items
}

// Get returns a single item by index.
func (s *Slice[T]) Get(i int) T {
	return (*s.items)[i]
}

// GetAll returns all items in the slice.
func (s *Slice[T]) GetAll() *[]T {
	return s.items
}

// Index returns the index of the item.
func (s *Slice[T]) Index(item T) int {
	return slices.Index(*s.items, item)
}

// Del removes an items from the slice.
func (s *Slice[T]) Del(item T) {
	idx := s.Index(item)
	if idx != -1 {
		*s.items = slices.Delete(*s.items, idx, idx+1)
	}
}

// TrimElements returns a new Slice object with the first len(elements) - n
// elements of the original slice.
func (s *Slice[T]) TrimElements(n int) *Slice[T] {
	var filtered []T
	if len(*s.items) > n {
		filtered = (*s.items)[:len(*s.items)-n]
	}

	return &Slice[T]{items: &filtered}
}

// Clean removes all items from the slice.
func (s *Slice[T]) Clean() {
	*s.items = make([]T, 0)
}

// From creates a new slice from the given items.
func From[T comparable](items []T) *Slice[T] {
	return &Slice[T]{items: &items}
}

// New creates a new slice of bookmarks.
func New[T comparable]() *Slice[T] {
	items := make([]T, 0)

	return &Slice[T]{items: &items}
}
