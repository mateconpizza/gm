package slice

import (
	"errors"
	"reflect"
	"testing"
)

var (
	intItems    = []int{1, 2, 3, 4, 5}
	strItems    = []string{"foo", "bar", "baz"}
	errExpected = errors.New("error: element is 3")
)

func TestForEach(t *testing.T) {
	s := New[int]()
	s.Add(&intItems[0])
	s.Add(&intItems[1])
	s.Add(&intItems[2])
	s.Add(&intItems[3])
	s.Add(&intItems[4])

	var sum int
	fn := func(i int) {
		sum += i
	}

	s.ForEach(fn)
	if sum != 15 {
		t.Errorf("expected sum of 15, got %d", sum)
	}
}

func TestForEachErr(t *testing.T) {
	s := New[int]()
	s.Add(&intItems[0])
	s.Add(&intItems[1])
	s.Add(&intItems[2])
	s.Add(&intItems[3])
	s.Add(&intItems[4])

	// Define a function that will be called for each element in the slice
	fn := func(i int) error {
		if i == 3 {
			return errExpected
		}

		return nil
	}

	// Call the ForEachErr function with the defined function
	err := s.ForEachErr(fn)

	// Check that the error is equal to the expected value
	if err == nil || err.Error() != "error: element is 3" {
		t.Errorf("expected error 'error: element is 3', got %v", err)
	}
}

func TestFilterInt(t *testing.T) {
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
		testT := test // Implicit memory aliasing in for loop.
		t.Run(test.name, func(t *testing.T) {
			s := New[int]()
			s.Set(&testT.input)
			s.Filter(testT.fn)
			if !reflect.DeepEqual(*s.items, testT.expected) {
				t.Errorf("expected %v, got %v", testT.expected, *s.items)
			}
		})
	}
}

func TestFilterStrings(t *testing.T) {
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
		testT := test // Implicit memory aliasing in for loop.
		t.Run(test.name, func(t *testing.T) {
			s := New[string]()
			s.Set(&testT.input)
			s.Filter(testT.fn)
			if !reflect.DeepEqual(*s.items, testT.expected) {
				t.Errorf("expected %v, got %v", testT.expected, *s.items)
			}
		})
	}
}

func TestIndexString(t *testing.T) {
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
			s := New[string]()
			s.Set(&test.input)
			idx := s.Index(test.name)
			if test.expected != idx {
				t.Errorf("expected %v, got %v", test.expected, idx)
			}
		})
	}
}

func TestDelete(t *testing.T) {
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
		s := New[string]()
		s.Set(&test.input)
		s.Del(test.name)

		if !reflect.DeepEqual(*s.items, test.expected) {
			t.Errorf("expected %v, got %v", test.expected, *s.items)
		}
	}
}

func TestAdd(t *testing.T) {
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
		s := New[string]()
		s.Set(&test.input)
		s.Add(&test.name)
		if !reflect.DeepEqual(*s.items, test.expected) {
			t.Errorf("expected %v, got %v", test.expected, *s.items)
		}
	}
}
