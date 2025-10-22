//nolint:paralleltest,funlen //using global colorEnabled
package frame

import (
	"bytes"
	"strings"
	"testing"
)

func containsANSICodes(t *testing.T, s string) bool {
	t.Helper()
	return strings.Contains(s, "\x1b[") || strings.Contains(s, "\033[")
}

func setupFrame(t *testing.T, opts []OptFn) *Frame {
	t.Helper()
	f := New(opts...)
	f.Text("Text\n").Textln("Text new line").
		Header("Header\n").Headerln("Header new line").
		Mid("Mid\n").Midln("Mid new line").
		Row("Row\n").Rowln("Row new line").
		Footer("Footer\n").Footerln("Footer new line").
		Warning("Warning\n").
		Info("Information\n").
		Error("Error!\n").
		Success("Success\n").
		Question("Ask question\n")

	return f
}

func TestFrame_String_ColorHandling(t *testing.T) {
	t.Run("color disabled excludes ANSI codes", func(t *testing.T) {
		DisableColor()
		defer func() {
			colorEnabled = true
		}()

		f := setupFrame(t, []OptFn{WithColorBorder(ColorBrightOrange)})
		output := f.String()

		if output == "" {
			t.Errorf("expected non-empty output, got empty string")
		}

		if containsANSICodes(t, output) {
			t.Errorf("expected no ANSI codes when color disabled, found in: %q", output)
		}
	})

	t.Run("color enabled includes ANSI codes", func(t *testing.T) {
		f := setupFrame(t, []OptFn{WithColorBorder(ColorBrightBlue)})
		output := f.String()

		if !containsANSICodes(t, output) {
			t.Errorf("expected ANSI color codes in output, got: %q", output)
		}
	})
}

func TestFrame_Writer(t *testing.T) {
	lines20 := `create mode 100644 somerepo/1.1.1.1/3HrYD_Ea5i4t.json
create mode 100644 somerepo/127.0.0.1/Iufqgcux-9RK.json
create mode 100644 somerepo/12factor.net/DKt21z9U73iw.json
create mode 100644 somerepo/1337x.to/ipWDvGKnpRAh.json
create mode 100644 somerepo/8192.one/aRk-QuEaDqRl.json
create mode 100644 somerepo/addictivetips.com/2pZgHlKVcicM.json
create mode 100644 somerepo/addictivetips.com/y4uO9Ra39Xua.json
create mode 100644 somerepo/addy-dclxvi.github.io/jGW4DpK-dGkp.json
create mode 100644 somerepo/agconti.github.io/L_yND2_WR7hm.json
create mode 100644 somerepo/alexedwards.net/NeIbaJfHV5fd.json
create mode 100644 somerepo/alexherbo2.github.io/3Znjt-46w_9a.json
create mode 100644 somerepo/anandology.com/LrxXARwCh1LX.json
create mode 100644 somerepo/angrynerd.in/nWOC9vBbZL5H.json
create mode 100644 somerepo/annas-archive.org/uxRmYXC801NG.json
create mode 100644 somerepo/archive.archlinux.org/Rq54ysGbHoZ3.json
create mode 100644 somerepo/arp242.net/bB1WxH02AsiE.json
create mode 100644 somerepo/arslan.io/cQVmSZtbC4Qe.json
create mode 100644 somerepo/askubuntu.com/RInmGTANazdz.json
create mode 100644 somerepo/askubuntu.com/kP1Y9q9yxmKy.json
create mode 100644 somerepo/atlassian.com/wxMO5eY45hTo.json`

	wantLines := 20
	lines := strings.Split(lines20, "\n")

	for range 100 {
		var buf bytes.Buffer
		f := New(WithColorBorder(ColorBrightBlue), WithWriter(&buf))
		f.Reset()

		for _, l := range lines {
			f.Rowln(l)
		}

		f.Flush()

		output := buf.String()
		outputLines := strings.Split(output, "\n")

		var gotLines []string
		for _, l := range outputLines {
			if l != "" {
				gotLines = append(gotLines, l)
			}
		}

		n := len(gotLines)
		if n != wantLines {
			t.Fatalf("expected %d lines, got %d, output:\n%v", wantLines, n, output)
		}
	}
}

func TestFrame_Write(t *testing.T) {
	tests := []struct {
		name     string
		writes   [][]byte
		expected []string // expected text elements after writes
	}{
		{
			name:     "single complete line",
			writes:   [][]byte{[]byte("hello world\n")},
			expected: []string{"| ", "hello world", "\n"},
		},
		{
			name:     "multiple complete lines",
			writes:   [][]byte{[]byte("line 1\nline 2\nline 3\n")},
			expected: []string{"| ", "line 1", "\n", "| ", "line 2", "\n", "| ", "line 3", "\n"},
		},
		{
			name: "incomplete line buffered",
			writes: [][]byte{
				[]byte("partial"),
			},
			expected: []string{}, // nothing written yet
		},
		{
			name: "buffered line completed",
			writes: [][]byte{
				[]byte("partial "),
				[]byte("line\n"),
			},
			expected: []string{"| ", "partial line", "\n"},
		},
		{
			name: "multiple writes with buffering",
			writes: [][]byte{
				[]byte("start"),
				[]byte(" middle"),
				[]byte(" end\n"),
				[]byte("next line\n"),
			},
			expected: []string{"| ", "start middle end", "\n", "| ", "next line", "\n"},
		},
		{
			name:     "empty lines ignored",
			writes:   [][]byte{[]byte("\n\n\n")},
			expected: []string{}, // empty lines are skipped
		},
		{
			name:     "mixed empty and non-empty lines",
			writes:   [][]byte{[]byte("line 1\n\nline 2\n")},
			expected: []string{"| ", "line 1", "\n", "| ", "line 2", "\n"},
		},
		{
			name:     "whitespace trimmed",
			writes:   [][]byte{[]byte("  spaced line  \n")},
			expected: []string{"| ", "spaced line", "\n"},
		},
		{
			name: "newline in middle of write",
			writes: [][]byte{
				[]byte("before\nafter"),
			},
			expected: []string{"| ", "before", "\n"}, // "after" stays in buffer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DisableColor()
			f := New()

			for _, write := range tt.writes {
				n, err := f.Write(write)
				if err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if n != len(write) {
					t.Errorf("Write() wrote %d bytes, want %d", n, len(write))
				}
			}

			if len(f.text) != len(tt.expected) {
				t.Errorf("got %d text elements, want %d", len(f.text), len(tt.expected))
				t.Logf("got: %#v", f.text)
				t.Logf("want: %#v", tt.expected)
				return
			}

			for i, want := range tt.expected {
				if f.text[i] != want {
					t.Errorf("text[%d] = %q, want %q", i, f.text[i], want)
				}
			}
		})
	}
}
