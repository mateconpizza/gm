//nolint:gocognit,err113,funlen //test
package slice

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

var (
	intItems = []int{1, 2, 3, 4, 5}
	strItems = []string{"foo", "bar", "baz"}
)

func TestForEach(t *testing.T) {
	t.Parallel()
	s := New(intItems...)
	result := 15
	var sum int
	s.ForEach(func(i int) {
		sum += i
	})
	if sum != result {
		t.Errorf("expected %v, got %v", result, sum)
	}
}

func TestForEachMut(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			s := &Slice[int]{items: tt.input}
			s.ForEachMut(tt.transform)
			if !reflect.DeepEqual(s.items, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, s.items)
			}
		})
	}
}

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
			t.Parallel()
			s := &Slice[int]{items: tt.input}
			err := s.ForEachErr(tt.fn)

			if tt.expectedErr != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedErr)
				} else if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error containing %q, got %q", tt.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
			if !reflect.DeepEqual(s.items, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, s.items)
			}
		})
	}
}

func TestForEachMutErr(t *testing.T) {
	t.Parallel()
	t.Run("mutates all elements successfully", func(t *testing.T) {
		t.Parallel()
		s := New(1, 2, 3)
		err := s.ForEachMutErr(func(n *int) error {
			*n++

			return nil
		})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		expected := []int{2, 3, 4}
		if !reflect.DeepEqual(s.items, expected) {
			t.Errorf("expected %v, got %v", expected, s.items)
		}
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

		if err == nil || !strings.Contains(err.Error(), "custom error") {
			t.Errorf("expected error containing 'custom error', got %v", err)
		}
		if callCount != 3 {
			t.Errorf("expected 3 calls, got %d", callCount)
		}
		expected := []int{2, 4, 3, 4}
		if !reflect.DeepEqual(s.items, expected) {
			t.Errorf("expected %v, got %v", expected, s.items)
		}
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

		if err == nil {
			t.Errorf("expected error, got nil")
		}
		expected := []Person{
			{"Alice (modified)", 30},
			{"Bob (modified)", 25},
			{"Charlie", 35},
		}
		if !reflect.DeepEqual(s.items, expected) {
			t.Errorf("expected %v, got %v", expected, s.items)
		}
	})

	t.Run("returns nil error for empty slice", func(t *testing.T) {
		t.Parallel()
		s := New[string]()
		err := s.ForEachMutErr(func(s *string) error {
			return errors.New("should never be called")
		})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if len(s.items) != 0 {
			t.Errorf("expected empty slice, got %v", s.items)
		}
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
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if (s.items)[0].Value != 10 {
			t.Errorf("expected first value to be 10, got %d", (s.items)[0].Value)
		}
		if (s.items)[1].Value != 20 {
			t.Errorf("expected second value to be 20, got %d", (s.items)[1].Value)
		}
		if (s.items)[2].Value != 30 {
			t.Errorf("expected third value to be 30, got %d", (s.items)[2].Value)
		}
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
			t.Parallel()
			s := New(test.input...)
			f := s.Filter(test.fn)
			if len(f.items) != len(test.expected) {
				t.Errorf("expected length %d, got %d", len(test.expected), len(f.items))
			}
			if !reflect.DeepEqual(f.items, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, f.items)
			}
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
			t.Parallel()
			s := New(test.input...)
			f := s.Filter(test.fn)
			if len(f.items) != len(test.expected) {
				t.Errorf("expected length %d, got %d", len(test.expected), len(f.items))
			}
			if !reflect.DeepEqual(f.items, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, f.items)
			}
		})
	}
}

func TestFilterInPlace(t *testing.T) {
	t.Run("basic filtering with integers", func(t *testing.T) {
		t.Parallel()
		s := New(1, 2, 3, 4, 5)
		originalCap := cap(s.items)

		s.FilterInPlace(func(n *int) bool {
			return *n%2 == 0
		})

		expected := []int{2, 4}
		if !reflect.DeepEqual(s.items, expected) {
			t.Errorf("expected %v, got %v", expected, s.items)
		}
		if len(s.items) != 2 {
			t.Errorf("expected length 2, got %d", len(s.items))
		}
		if cap(s.items) != originalCap {
			t.Errorf("expected capacity %d, got %d (capacity should be preserved)", originalCap, cap(s.items))
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		t.Parallel()
		s := New[int]()
		s.FilterInPlace(func(n *int) bool { return true })
		if len(s.items) != 0 {
			t.Errorf("expected empty slice, got %v", s.items)
		}
	})

	t.Run("no elements filtered", func(t *testing.T) {
		t.Parallel()
		s := New("apple", "banana", "cherry")
		original := make([]string, len(s.items))
		copy(original, s.items)

		s.FilterInPlace(func(s *string) bool {
			return true // Keep all
		})

		if !reflect.DeepEqual(s.items, original) {
			t.Errorf("expected %v, got %v", original, s.items)
		}
		if len(s.items) != 3 {
			t.Errorf("expected length 3, got %d", len(s.items))
		}
	})

	t.Run("all elements filtered", func(t *testing.T) {
		t.Parallel()
		s := New(1.1, 2.2, 3.3)
		s.FilterInPlace(func(f *float64) bool {
			return *f > 10.0 // None match
		})

		if len(s.items) != 0 {
			t.Errorf("expected empty slice, got %v", s.items)
		}
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

		if len(s.items) != 2 {
			t.Errorf("expected length 2, got %d", len(s.items))
		}

		found := make(map[Person]bool)
		for _, person := range s.items {
			found[person] = true
		}

		expected := []Person{{"Alice", 30}, {"Charlie", 35}}
		for _, person := range expected {
			if !found[person] {
				t.Errorf("expected to find %v in result", person)
			}
		}
	})

	t.Run("pointer elements", func(t *testing.T) {
		t.Parallel()
		ptr := func(v int) *int { return &v }
		s := New(ptr(1), ptr(2), ptr(3), ptr(4))

		s.FilterInPlace(func(n **int) bool {
			return **n > 2
		})

		if len(s.items) != 2 {
			t.Errorf("expected length 2, got %d", len(s.items))
		}
		if *(s.items)[0] != 3 {
			t.Errorf("expected first element to be 3, got %d", *(s.items)[0])
		}
		if *(s.items)[1] != 4 {
			t.Errorf("expected second element to be 4, got %d", *(s.items)[1])
		}
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

		expected := []int{4, 5}
		if !reflect.DeepEqual(s.items, expected) {
			t.Errorf("expected %v, got %v", expected, s.items)
		}
		if len(s.items) != 2 {
			t.Errorf("expected length 2, got %d", len(s.items))
		}
		if cap(s.items) != 5 {
			t.Errorf("expected capacity 5, got %d (capacity should remain unchanged)", cap(s.items))
		}
	})

	t.Run("mutation check", func(t *testing.T) {
		t.Parallel()
		s := New(1, 2, 3)
		originalPtr := &(s.items)[0]

		s.FilterInPlace(func(n *int) bool {
			*n *= 10 // Mutate values during filtering
			return *n > 15
		})

		expected := []int{20, 30}
		if !reflect.DeepEqual(s.items, expected) {
			t.Errorf("expected %v, got %v", expected, s.items)
		}
		if originalPtr != &(s.items)[0] {
			t.Error("underlying array should be the same")
		}
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
			t.Parallel()
			s := New(test.input...)
			idx := s.Index(test.name)
			if idx == -1 {
				t.Errorf("expected to find %q, but got -1", test.name)
			}
			if s.items[idx] != test.name {
				t.Errorf("expected %q at index %d, got %q", test.name, idx, s.items[idx])
			}
			if idx != test.expected {
				t.Errorf("expected index %v, got %v", test.expected, idx)
			}
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
		if len(s.items) != len(test.expected) {
			t.Errorf("expected length %d, got %d", len(test.expected), len(s.items))
		}
		if !reflect.DeepEqual(s.items, test.expected) {
			t.Errorf("expected %v, got %v", test.expected, s.items)
		}
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
		if !reflect.DeepEqual(s.items, test.expected) {
			t.Errorf("expected %v, got %v", test.expected, s.items)
		}
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
			t.Parallel()
			s := &Slice[int]{items: tt.items}
			result := s.Any(tt.predicate)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIncludes(t *testing.T) {
	t.Parallel()
	t.Skip("not implemented yet")
}
