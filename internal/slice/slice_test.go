package slice

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	intItems = []int{1, 2, 3, 4, 5}
	strItems = []string{"foo", "bar", "baz"}
)

func TestForEach(t *testing.T) {
	s := New(intItems...)
	result := 15
	var sum int
	s.ForEach(func(i int) {
		sum += i
	})
	assert.Equal(t, result, sum, "expected %v, got %v", result, sum)
}

func TestForEachMut(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		transform func(*int)
		expected  []int
	}{
		{
			name:  "empty slice",
			input: []int{},
			transform: func(i *int) {
				*i *= 2
			},
			expected: []int{},
		},
		{
			name:  "no change",
			input: []int{1, 2, 3},
			transform: func(i *int) {
				// No change
			},
			expected: []int{1, 2, 3},
		},
		{
			name:  "multiply by 2",
			input: []int{1, 2, 3},
			transform: func(i *int) {
				*i *= 2
			},
			expected: []int{2, 4, 6},
		},
		{
			name:  "add 1",
			input: []int{1, 2, 3},
			transform: func(i *int) {
				*i++
			},
			expected: []int{2, 3, 4},
		},
		{
			name:  "mixed operations",
			input: []int{1, 2, 3},
			transform: func(i *int) {
				if *i%2 == 0 {
					*i *= 3
				} else {
					*i += 5
				}
			},
			expected: []int{6, 6, 8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Slice[int]{items: tt.input}
			s.ForEachMut(tt.transform)
			assert.Equal(t, tt.expected, s.items)
		})
	}
}

//nolint:funlen,err113 //test
func TestForEachErr(t *testing.T) {
	t.Parallel()
	var (
		errExpected  = errors.New("first element error")
		errExpected2 = errors.New("second element error")
		errExpected3 = errors.New("always error")
	)
	type testCase[T any] struct {
		name        string
		input       []T
		fn          func(T) error
		expected    []T
		expectedErr string
	}

	tests := []testCase[int]{
		{
			name:  "no error",
			input: []int{1, 2, 3},
			fn: func(i int) error {
				return nil
			},
			expected:    []int{1, 2, 3},
			expectedErr: "",
		},
		{
			name:  "error on first element",
			input: []int{0, 1, 2, 3},
			fn: func(i int) error {
				if i == 0 {
					return errExpected
				}

				return nil
			},
			expected:    []int{0, 1, 2, 3},
			expectedErr: errExpected.Error(),
		},
		{
			name:  "error on second element",
			input: []int{1, 2, 3},
			fn: func(i int) error {
				if i == 2 {
					return errExpected2
				}

				return nil
			},
			expected:    []int{1, 2, 3},
			expectedErr: "second element error",
		},
		{
			name:  "always error",
			input: []int{1, 2, 3},
			fn: func(i int) error {
				return errExpected3
			},
			expected:    []int{1, 2, 3},
			expectedErr: "always error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Slice[int]{items: tt.input}
			err := s.ForEachErr(tt.fn)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, s.items)
		})
	}
}

//nolint:funlen,err113 //test
func TestForEachMutErr(t *testing.T) {
	t.Parallel()
	t.Run("mutates all elements successfully", func(t *testing.T) {
		t.Parallel()
		s := New(1, 2, 3)
		err := s.ForEachMutErr(func(n *int) error {
			*n++

			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, []int{2, 3, 4}, s.items)
	})

	t.Run("Stops on error and returns partial mutations", func(t *testing.T) {
		t.Parallel()
		s := New(1, 2, 3, 4)
		callCount := 0
		expectedError := errors.New("custom error")
		err := s.ForEachMutErr(func(n *int) error {
			callCount++
			if *n == 3 {
				return expectedError
			}
			*n *= 2

			return nil
		})

		assert.ErrorContains(t, err, "custom error")
		assert.Equal(t, 3, callCount)
		assert.Equal(t, []int{2, 4, 3, 4}, s.items)
	})

	t.Run("handles struct mutations with errors", func(t *testing.T) {
		t.Parallel()
		type Person struct {
			Name string
			Age  int
		}
		s := New(
			Person{"Alice", 30},
			Person{"Bob", 25},
			Person{"Charlie", 35},
		)
		err := s.ForEachMutErr(func(p *Person) error {
			if p.Age > 30 {
				return errors.New("age limit exceeded")
			}
			p.Name += " (modified)"

			return nil
		})

		assert.Error(t, err)
		assert.Equal(t, []Person{
			{"Alice (modified)", 30},
			{"Bob (modified)", 25},
			{"Charlie", 35},
		}, s.items)
	})

	t.Run("returns nil error for empty slice", func(t *testing.T) {
		t.Parallel()
		s := New[string]()
		err := s.ForEachMutErr(func(s *string) error {
			return errors.New("should never be called")
		})

		assert.NoError(t, err)
		assert.Empty(t, s.items)
	})

	t.Run("mutation with pointer types", func(t *testing.T) {
		t.Parallel()
		type Counter struct {
			Value int
		}

		s := New(
			&Counter{1},
			&Counter{2},
			&Counter{3},
		)

		err := s.ForEachMutErr(func(c **Counter) error {
			(*c).Value *= 10
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 10, (s.items)[0].Value)
		assert.Equal(t, 20, (s.items)[1].Value)
		assert.Equal(t, 30, (s.items)[2].Value)
	})
}

func TestFilterInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		fn       func(int) bool
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "even numbers",
			input:    intItems,
			expected: []int{2, 4},
			fn:       func(i int) bool { return i%2 == 0 },
		},
		{
			name:     "odd numbers",
			input:    intItems,
			expected: []int{1, 3, 5},
			fn:       func(i int) bool { return i%2 == 1 },
		},
		{
			name:     "all numbers",
			input:    intItems,
			expected: []int{1, 2, 3, 4, 5},
			fn:       func(i int) bool { return true },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := New(test.input...)
			f := s.Filter(test.fn)
			assert.Equal(t, len(f.items), len(test.expected))
			assert.Equal(
				t,
				test.expected,
				f.items,
				"expected %v, got %v",
				test.expected,
				f.items,
			)
		})
	}
}

func TestFilterStrings(t *testing.T) {
	t.Parallel()
	testWithInt := []struct {
		fn       func(string) bool
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "bar",
			input:    strItems,
			expected: []string{"bar"},
			fn:       func(s string) bool { return s == "bar" },
		},
		{
			name:     "all strings",
			input:    strItems,
			expected: []string{"foo", "bar", "baz"},
			fn:       func(s string) bool { return len(s) == 3 },
		},
	}

	for _, test := range testWithInt {
		t.Run(test.name, func(t *testing.T) {
			s := New(test.input...)
			f := s.Filter(test.fn)
			assert.Equal(t, len(f.items), len(test.expected))
			assert.Equal(
				t,
				test.expected,
				f.items,
				"expected %v, got %v",
				test.expected,
				f.items,
			)
		})
	}
}

//nolint:funlen //test
func TestFilterInPlace(t *testing.T) {
	t.Run("basic filtering with integers", func(t *testing.T) {
		t.Parallel()
		s := New(1, 2, 3, 4, 5)
		originalCap := cap(s.items)

		s.FilterInPlace(func(n *int) bool {
			return *n%2 == 0
		})

		assert.Equal(t, []int{2, 4}, s.items)
		assert.Len(t, s.items, 2)
		assert.Equal(t, originalCap, cap(s.items), "capacity should be preserved")
	})

	t.Run("empty slice", func(t *testing.T) {
		t.Parallel()
		s := New[int]()
		s.FilterInPlace(func(n *int) bool { return true })
		assert.Empty(t, s.items)
	})

	t.Run("no elements filtered", func(t *testing.T) {
		t.Parallel()
		s := New("apple", "banana", "cherry")
		original := s.items

		s.FilterInPlace(func(s *string) bool {
			return true // Keep all
		})

		assert.Equal(t, original, s.items)
		assert.Len(t, s.items, 3)
	})

	t.Run("all elements filtered", func(t *testing.T) {
		t.Parallel()
		s := New(1.1, 2.2, 3.3)
		s.FilterInPlace(func(f *float64) bool {
			return *f > 10.0 // None match
		})

		assert.Empty(t, s.items)
	})

	t.Run("struct filtering", func(t *testing.T) {
		t.Parallel()
		type Person struct {
			Name string
			Age  int
		}

		s := New(
			Person{"Alice", 30},
			Person{"Bob", 25},
			Person{"Charlie", 35},
		)

		s.FilterInPlace(func(p *Person) bool {
			return p.Age >= 30
		})

		assert.Len(t, s.items, 2)
		assert.Contains(t, s.items, Person{"Alice", 30})
		assert.Contains(t, s.items, Person{"Charlie", 35})
	})

	t.Run("pointer elements", func(t *testing.T) {
		t.Parallel()
		ptr := func(v int) *int { return &v }
		s := New(ptr(1), ptr(2), ptr(3), ptr(4))

		s.FilterInPlace(func(n **int) bool {
			return **n > 2
		})

		assert.Len(t, s.items, 2)
		assert.Equal(t, 3, *(s.items)[0])
		assert.Equal(t, 4, *(s.items)[1])
	})

	t.Run("capacity management", func(t *testing.T) {
		t.Parallel()
		// Slice inicial con capacidad 5
		original := make([]int, 0, 5)
		original = append(original, 1, 2, 3, 4, 5)
		s := &Slice[int]{items: original}

		s.FilterInPlace(func(n *int) bool {
			return *n > 3
		})

		assert.Equal(t, []int{4, 5}, s.items)
		assert.Len(t, s.items, 2)
		assert.Equal(t, 5, cap(s.items), "capacity should remain unchanged")
	})

	t.Run("mutation check", func(t *testing.T) {
		t.Parallel()
		s := New(1, 2, 3)
		originalPtr := &(s.items)[0]

		s.FilterInPlace(func(n *int) bool {
			*n *= 10 // Mutate values during filtering
			return *n > 15
		})

		assert.Equal(t, []int{20, 30}, s.items)
		assert.Same(t, originalPtr, &(s.items)[0], "underlying array should be the same")
	})
}

func TestIndexString(t *testing.T) {
	t.Parallel()
	testIdx := []struct {
		name     string
		input    []string
		expected int
	}{
		{
			name:     "foo",
			input:    strItems,
			expected: 0,
		},
		{
			name:     "bar",
			input:    strItems,
			expected: 1,
		},
		{
			name:     "baz",
			input:    strItems,
			expected: 2,
		},
	}

	for _, test := range testIdx {
		t.Run(test.name, func(t *testing.T) {
			s := New(test.input...)
			idx := s.Index(test.name)
			assert.NotEqual(t, idx, -1)
			assert.Equal(t, s.items[idx], test.name)
			assert.Equal(t, test.expected, idx, "expected %v, got %v", test.expected, idx)
		})
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	testDeleteStr := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "bar",
			input:    []string{"foo", "bar", "baz"},
			expected: []string{"foo", "baz"},
		},
		{
			name:     "foo",
			input:    []string{"foo", "bar", "baz"},
			expected: []string{"bar", "baz"},
		},
		{
			name:     "baz",
			input:    []string{"foo", "bar", "baz"},
			expected: []string{"foo", "bar"},
		},
	}

	for _, test := range testDeleteStr {
		s := New(test.input...)
		s.Del(test.name)
		assert.Equal(t, len(s.items), len(test.expected))
		assert.Equal(t, test.expected, s.items, "expected %v, got %v", test.expected, s.items)
	}
}

func TestAppendLoop(t *testing.T) {
	t.Parallel()
	testAddStr := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "foo",
			input:    []string{"foo"},
			expected: []string{"foo"},
		},
		{
			name:     "bar",
			input:    []string{"foo", "baz"},
			expected: []string{"foo", "baz", "bar"},
		},
		{
			name:     "baz",
			input:    []string{},
			expected: []string{"baz"},
		},
	}

	for _, test := range testAddStr {
		s := New(test.input...)
		s.Append(test.name)
		assert.Equal(t, test.expected, s.items, "expected %v, got %v", test.expected, s.items)
	}
}

func TestAny(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		items     []int
		predicate func(int) bool
		expected  bool
	}{
		{
			name:      "at least one element matches",
			items:     []int{1, 2, 3},
			predicate: func(i int) bool { return i%2 == 0 },
			expected:  true,
		},
		{
			name:      "no elements match",
			items:     []int{1, 3, 5},
			predicate: func(i int) bool { return i%2 == 0 },
			expected:  false,
		},
		{
			name:      "empty slice",
			items:     []int{},
			predicate: func(i int) bool { return true },
			expected:  false,
		},
		{
			name:      "first element matches",
			items:     []int{2, 1, 3},
			predicate: func(i int) bool { return i%2 == 0 },
			expected:  true,
		},
		{
			name:      "all elements match",
			items:     []int{2, 4, 6},
			predicate: func(i int) bool { return i%2 == 0 },
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Slice[int]{items: tt.items}
			assert.Equal(t, tt.expected, s.Any(tt.predicate))
		})
	}
}

func TestIncludes(t *testing.T) {
	t.Parallel()
	t.Skip("not implemented yet")
}
