package slice

type Slice[T any] struct {
	items *[]T
}

// ForEach loops all items
func (s *Slice[T]) ForEach(fn func(T)) {
	for _, ele := range *s.items {
		fn(ele)
	}
}

// ForEachErr loops all items and returns a err
func (s *Slice[T]) ForEachErr(fn func(T) error) error {
	for _, ele := range *s.items {
		if err := fn(ele); err != nil {
			return err
		}
	}
	return nil
}

// ForEachIdx loop items all items with index
func (s *Slice[T]) ForEachIdx(fn func(int, T) error) error {
	for i, ele := range *s.items {
		if err := fn(i, ele); err != nil {
			return err
		}
	}
	return nil
}

// Filter
func (s *Slice[T]) Filter(fn func(T) bool) {
	var slice = New[T]()
	for _, b := range *s.items {
		t := b
		if fn(t) {
			slice.Add(&t)
		}
	}
	*s.items = *slice.items
}

func (s *Slice[T]) Has(fn func(T) bool) bool {
	for _, ele := range *s.items {
		if fn(ele) {
			return true
		}
	}
	return false
}

func (s *Slice[T]) Head(n int) {
	l := s.Len()
	if n <= 0 || n > l || l == 0 {
		return
	}
	*s.items = (*s.items)[:n]
}

func (s *Slice[T]) Tail(n int) {
	l := s.Len()
	if n <= 0 || n > l || l == 0 {
		return
	}
	tail := l - n
	*s.items = (*s.items)[tail:]
}

func (s *Slice[T]) Len() int {
	return len(*s.items)
}

// Add adds a single item to the items
func (s *Slice[T]) Add(b *T) {
	*s.items = append(*s.items, *b)
}

// Set sets the items
func (s *Slice[T]) Set(items *[]T) {
	*s.items = *items
}

// Get returns a single item by index
func (s *Slice[T]) Get(i int) T {
	return (*s.items)[i]
}

// GetAll returns all items in the slice
func (s *Slice[T]) GetAll() *[]T {
	return s.items
}

// New creates a new slice of bookmarks
func New[T any]() *Slice[T] {
	items := make([]T, 0)
	return &Slice[T]{items: &items}
}
