package terminal

import (
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/ui/color"
)

func TestTermGetUserInput(t *testing.T) {
	t.Run("confirm", func(t *testing.T) {
		t.Parallel()

		opts := []string{"yes", "no"}
		input := "yes\n"
		mockInput := strings.NewReader(input)
		mockOutput := &strings.Builder{}

		result, err := getUserInputWithAttempts(mockInput, mockOutput, "Proceed?", opts, "no")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "yes" {
			t.Errorf("expected result 'yes', got '%s'", result)
		}
	})

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		opts := []string{"yes", "no"}
		input := "\n"
		mockInput := strings.NewReader(input)
		mockOutput := &strings.Builder{}

		result, err := getUserInputWithAttempts(mockInput, mockOutput, "Proceed?", opts, "no")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "no" {
			t.Errorf("expected result 'no', got '%s'", result)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		opts := []string{"yes", "no"}
		input := "invalid\n"
		mockInput := strings.NewReader(input)
		mockOutput := &strings.Builder{}

		result, err := getUserInputWithAttempts(mockInput, mockOutput, "Proceed?", opts, "no")
		if err == nil {
			t.Fatal("expected error but got nil")
		}
		if result != "" {
			t.Errorf("expected empty result, got '%s'", result)
		}
	})
}

func TestTermGetQueryFromPipe(t *testing.T) {
	t.Parallel()

	input := "hello\n"
	mockInput := strings.NewReader(input)
	result := getQueryFromPipe(mockInput)
	if input != result {
		t.Fatalf("expected '%s', got '%s'", input, result)
	}
}

func TestTermFmtChoicesWithDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []string
		def  string
		want []string
	}{
		{
			name: "with default 'no'",
			opts: []string{"yes", "no"},
			def:  "n",
			want: []string{"yes", "No"},
		},
		{
			name: "with default 'yes'",
			opts: []string{"yes", "no"},
			def:  "y",
			want: []string{"no", "Yes"},
		},
		{
			name: "no default",
			opts: []string{"yes", "no"},
			def:  "",
			want: []string{"yes", "no"},
		},
		{
			name: "empty options",
			opts: []string{},
			def:  "",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := fmtChoicesWithDefaultColor(tt.opts, tt.def)
			for i := range len(result) {
				result[i] = color.ANSICodeRemover(result[i])
			}

			if len(result) != len(tt.want) {
				t.Fatalf("expected %d elements, got %d", len(tt.want), len(result))
			}
			for i := range tt.want {
				if result[i] != tt.want[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.want[i], result[i])
				}
			}
		})
	}
}
