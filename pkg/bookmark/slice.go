package bookmark

type Slice struct {
	items *[]Bookmark
}

// ForEach loops all items
func (s *Slice) ForEach(fn func(Bookmark)) {
	for _, ele := range *s.items {
		fn(ele)
	}
}

// ForEachErr loops all items and returns a err
func (s *Slice) ForEachErr(fn func(Bookmark) error) error {
	for _, ele := range *s.items {
		if err := fn(ele); err != nil {
			return err
		}
	}
	return nil
}

// ForEachIdx loop items all items with index
func (s *Slice) ForEachIdx(fn func(int, Bookmark) error) error {
	for i, ele := range *s.items {
		if err := fn(i, ele); err != nil {
			return err
		}
	}
	return nil
}

func (s *Slice) Has(fn func(Bookmark) bool) bool {
	for _, ele := range *s.items {
		if fn(ele) {
			return true
		}
	}
	return false
}

func (s *Slice) Head(n int) {
	l := s.Len()
	if n <= 0 || n > l || l == 0 {
		return
	}
	*s.items = (*s.items)[:n]
}

func (s *Slice) Tail(n int) {
	l := s.Len()
	if n <= 0 || n > l || l == 0 {
		return
	}
	tail := l - n
	*s.items = (*s.items)[tail:]
}

func (s *Slice) Len() int {
	return len(*s.items)
}

// Add adds a single item to the items
func (s *Slice) Add(b *Bookmark) {
	*s.items = append(*s.items, *b)
}

// Set sets the items
func (s *Slice) Set(items *[]Bookmark) {
	*s.items = *items
}

// Get returns a single item by index
func (s *Slice) Get(i int) Bookmark {
	return (*s.items)[i]
}

// GetAll returns all items in the slice
func (s *Slice) GetAll() *[]Bookmark {
	return s.items
}

// IDs returns all items IDs
func (s *Slice) IDs() []int {
	ids := make([]int, 0, len(*s.items))
	for _, b := range *s.items {
		ids = append(ids, b.ID)
	}
	return ids
}

// NewSlice creates a new slice of bookmarks
func NewSlice() *Slice {
	return &Slice{items: &[]Bookmark{}}
}
