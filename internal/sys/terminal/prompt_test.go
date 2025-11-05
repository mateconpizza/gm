package terminal

import (
	"strings"
	"testing"

	"github.com/mateconpizza/gm/pkg/ansi"
)

func TestTermGetUserInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		options    []string
		prompt     string
		defaultVal string
		wantResult string
		wantErr    bool
	}{
		{
			name:       "confirm with yes",
			input:      "yes\n",
			options:    []string{"yes", "no"},
			prompt:     "Proceed?",
			defaultVal: "no",
			wantResult: "yes",
			wantErr:    false,
		},
		{
			name:       "confirm with no",
			input:      "no\n",
			options:    []string{"yes", "no"},
			prompt:     "Proceed?",
			defaultVal: "yes",
			wantResult: "no",
			wantErr:    false,
		},
		{
			name:       "use default when empty input",
			input:      "\n",
			options:    []string{"yes", "no"},
			prompt:     "Proceed?",
			defaultVal: "no",
			wantResult: "no",
			wantErr:    false,
		},
		{
			name:       "use default with yes",
			input:      "\n",
			options:    []string{"yes", "no"},
			prompt:     "Proceed?",
			defaultVal: "yes",
			wantResult: "yes",
			wantErr:    false,
		},
		{
			name:       "invalid option returns error",
			input:      "invalid\n",
			options:    []string{"yes", "no"},
			prompt:     "Proceed?",
			defaultVal: "no",
			wantResult: "",
			wantErr:    true,
		},
		{
			name:       "invalid option multiple attempts",
			input:      "maybe\ninvalid\nwrong\n",
			options:    []string{"yes", "no"},
			prompt:     "Proceed?",
			defaultVal: "no",
			wantResult: "",
			wantErr:    true,
		},
		{
			name:       "case insensitive input",
			input:      "YES\n",
			options:    []string{"yes", "no"},
			prompt:     "Proceed?",
			defaultVal: "no",
			wantResult: "yes",
			wantErr:    false,
		},
		{
			name:       "whitespace trimmed",
			input:      "  yes  \n",
			options:    []string{"yes", "no"},
			prompt:     "Proceed?",
			defaultVal: "no",
			wantResult: "yes",
			wantErr:    false,
		},
		{
			name:       "single character options",
			input:      "y\n",
			options:    []string{"y", "n"},
			prompt:     "Continue?",
			defaultVal: "n",
			wantResult: "y",
			wantErr:    false,
		},
		{
			name:       "multiple options with different default",
			input:      "cancel\n",
			options:    []string{"save", "discard", "cancel"},
			prompt:     "What to do?",
			defaultVal: "save",
			wantResult: "cancel",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockPromptInput := &PromptInput{
				Reader:  strings.NewReader(tt.input),
				Writer:  &strings.Builder{},
				Prompt:  tt.prompt,
				Options: tt.options,
				Default: tt.defaultVal,
			}

			result, err := getUserInputWithAttempts(mockPromptInput)

			if (err != nil) != tt.wantErr {
				t.Errorf("getUserInputWithAttempts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result != tt.wantResult {
				t.Errorf("getUserInputWithAttempts() = %q, want %q", result, tt.wantResult)
			}
		})
	}
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
				result[i] = ansi.Remover(result[i])
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
