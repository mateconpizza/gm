package menu

import (
	"testing"

	fzf "github.com/junegunn/fzf/src"
	"github.com/stretchr/testify/assert"
)

type fakeRunner struct {
	retcode int
	output  string
}

func (f *fakeRunner) Parse(defaults bool, settings FzfSettings) (*fzf.Options, error) {
	return &fzf.Options{}, nil
}

func (f *fakeRunner) Run(opts *fzf.Options) (int, error) {
	opts.Output <- f.output
	return f.retcode, nil
}

//nolint:funlen //test
func TestSelectReturnsSelectedItem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		items    []any
		output   string
		expected []any
		recode   int
		err      error
	}{
		{
			name:     "strings",
			items:    []any{"x", "y", "z"},
			output:   "y",
			expected: []any{"y"},
			recode:   0,
		},
		{
			name:     "ints",
			items:    []any{1, 2, 3},
			output:   "3",
			expected: []any{3},
			recode:   0,
		},
		{
			name:     "no items",
			items:    []any{},
			output:   "",
			expected: nil,
			recode:   1,
			err:      ErrFzfNoItems,
		},
		{
			name:     "no match",
			items:    []any{1, 2},
			output:   "",
			expected: nil,
			recode:   1,
			err:      ErrFzfNoMatching,
		},
		{
			name:     "action aborted",
			items:    []any{1, 2},
			output:   "",
			expected: nil,
			recode:   130,
			err:      ErrFzfActionAborted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := make([]any, len(tt.items))
			copy(s, tt.items)

			r := &fakeRunner{output: tt.output, retcode: tt.recode}
			m := New[any](WithRunner(r))
			m.SetItems(s)
			m.SetPreprocessor(defaultPreprocessor)
			result, err := m.Select()

			if tt.recode != 0 {
				assert.Nil(t, result)
				assert.ErrorIs(t, err, tt.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
