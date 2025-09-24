//nolint:funlen //test
package txt

import (
	"slices"
	"strings"
	"testing"
)

func TestExtractBtock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		startMarker string
		endMarker   string
		expected    string
		content     []string
	}{
		{
			name: "Normal case with start and end",
			content: []string{
				"Line 1",
				"## start marker",
				"Content to extract",
				"More content",
				"## end marker",
				"Line after end marker",
			},
			startMarker: "## start marker",
			endMarker:   "## end marker",
			expected:    "Content to extract\nMore content",
		},
		{
			name: "No start marker returns empty",
			content: []string{
				"Line 1",
				"Random line",
				"## end marker",
			},
			startMarker: "## start marker",
			endMarker:   "## end marker",
			expected:    "",
		},
		{
			name: "Empty end marker extracts until EOF",
			content: []string{
				"header",
				"### START",
				"line 1",
				"line 2",
				"line 3",
			},
			startMarker: "### START",
			endMarker:   "",
			expected:    "line 1\nline 2\nline 3",
		},
		{
			name: "Trims leading and trailing blank lines inside block",
			content: []string{
				"header",
				"### START",
				"",
				"   ",
				"real content",
				"",
				"   ",
				"### END",
				"footer",
			},
			startMarker: "### START",
			endMarker:   "### END",
			expected:    "real content",
		},
		{
			name: "Start and end markers adjacent yields empty block",
			content: []string{
				"### START",
				"### END",
				"footer",
			},
			startMarker: "### START",
			endMarker:   "### END",
			expected:    "",
		},
		{
			name: "Multiple possible blocks, only first is extracted",
			content: []string{
				"### START",
				"first block",
				"### END",
				"### START",
				"second block",
				"### END",
			},
			startMarker: "### START",
			endMarker:   "### END",
			expected:    "first block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ExtractBlock(tt.content, tt.startMarker, tt.endMarker)
			if result != tt.expected {
				t.Errorf(
					"Failed for test %q\nExpected: %q\nGot: %q",
					tt.name, tt.expected, result,
				)
			}
		})
	}
}

func FuzzExtractBlock(f *testing.F) {
	// Seed corpus
	f.Add([]byte("## start\nfoo\nbar\n## end\n"), "## start", "## end")
	f.Add([]byte("no markers here"), "## start", "## end")
	f.Add([]byte("START\nline\nEND\n"), "START", "END")
	f.Add([]byte("   \n\t\n"), "X", "Y")

	f.Fuzz(func(t *testing.T, content []byte, startMarker, endMarker string) {
		lines := strings.Split(string(content), "\n")
		result := ExtractBlock(lines, startMarker, endMarker)

		// Invariant 1: result must not contain markers
		if strings.Contains(result, startMarker) {
			t.Errorf("result contains startMarker: %q in %q", startMarker, result)
		}
		if endMarker != "" && strings.Contains(result, endMarker) {
			t.Errorf("result contains endMarker: %q in %q", endMarker, result)
		}

		// Invariant 2: if result not empty, all lines must come from content
		if result != "" {
			lines := strings.SplitSeq(result, "\n")
			for l := range lines {
				found := slices.Contains(strings.Split(string(content), "\n"), l)
				if !found {
					t.Errorf("line %q in result not found in original content %q", l, content)
				}
			}
		}
	})
}

func TestDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		inputA   []byte
		inputB   []byte
		expected string
	}{
		{
			name:     "No Changes",
			inputA:   []byte("line1\nline2\nline3"),
			inputB:   []byte("line1\nline2\nline3"),
			expected: "line1\nline2\nline3",
		},
		{
			name:     "Line Added",
			inputA:   []byte("line1\nline2"),
			inputB:   []byte("line1\nline2\nline3"),
			expected: "line1\nline2\n+line3",
		},
		{
			name:     "Line Removed",
			inputA:   []byte("line1\nline2\nline3"),
			inputB:   []byte("line1\nline3"),
			expected: "line1\n-line2\nline3",
		},
		{
			name:     "Line Modified",
			inputA:   []byte("line1\nline2\nline3"),
			inputB:   []byte("line1\nlineX\nline3"),
			expected: "line1\n-line2\n+lineX\nline3",
		},
		{
			name:     "Multiple Changes",
			inputA:   []byte("lineA\nlineB\nlineC\nlineD"),
			inputB:   []byte("lineA\nlineX\nlineC\nlineY"),
			expected: "lineA\n-lineB\n+lineX\nlineC\n-lineD\n+lineY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := Diff(tt.inputA, tt.inputB)
			if result != tt.expected {
				t.Errorf("Diff() = %q, expected %q", result, tt.expected)
			}
		})
	}
}
