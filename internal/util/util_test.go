package util

import (
	"log"
	"os"
	"reflect"
	"testing"
)

func TestExtractID(t *testing.T) {
	tests := []struct {
		line     string
		expected int
	}{
		{
			line:     "7  · https://www.thetvdb.com     et,multimedia,database,",
			expected: 7,
		},
		{
			line:     "483 - https://www.example.org",
			expected: 483,
		},
		{
			line:     "XXX - https://www.example.org",
			expected: -1,
		},
		{
			line:     "1 - https://www.example.org",
			expected: 1,
		},
	}

	for _, tt := range tests {
		got := ExtractID(tt.line)
		if got != tt.expected {
			t.Errorf("got: %v, expected: %d", got, tt.expected)
		}
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
	}{
		{
			name:  "EnvSet",
			env:   "TEST_KEY",
			value: "testValue",
		},
		{
			name:  "EnvNotSet",
			env:   "TEST_KEY",
			value: "",
		},
		{
			name:  "EnvOne",
			env:   "TEST_KEY",
			value: "1",
		},
		{
			name:  "",
			env:   "",
			value: "",
		},
	}

	for _, tt := range tests {
		_ = os.Setenv(tt.env, tt.value)
		got := GetEnv(tt.env, tt.value)

		// unset env after obtaining value
		if err := os.Unsetenv(tt.env); err != nil {
			log.Printf("unseting env: %s", err.Error())
		}

		if got != tt.value {
			t.Errorf("%s: got: %v, expected: %s", tt.name, got, tt.value)
		}
	}
}

func TestParseUniqueStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      string
		expected []string
	}{
		{
			name:     "comma separated",
			input:    "foo,bar,baz",
			sep:      ",",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "dash separated",
			input:    "foo-bar-bar",
			sep:      "-",
			expected: []string{"foo", "bar"},
		},
		{
			name:     "mid separated",
			input:    "bar·bar·bar",
			sep:      "·",
			expected: []string{"bar"},
		},
		{
			name:     "slash separated",
			input:    "bar/baz/bar/foo",
			sep:      "/",
			expected: []string{"bar", "baz", "foo"},
		},
	}
	for _, tt := range tests {
		got := ParseUniqueStrings(tt.input, tt.sep)

		if !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("%s: got: %v, expected: %v", tt.name, got, tt.expected)
		}
	}
}

func TestBinExists(t *testing.T) {
	tests := []struct {
		command string
		exists  bool
	}{
		{
			command: "awk",
			exists:  true,
		},
		{
			command: "cat",
			exists:  true,
		},
		{
			command: "uname",
			exists:  true,
		},
		{
			command: "dotnotexists",
			exists:  false,
		},
		{
			command: "anotherone",
			exists:  false,
		},
	}

	for _, tt := range tests {
		got := BinExists(tt.command)
		if got != tt.exists {
			t.Errorf("%s: got: %v, expected: %v", tt.command, got, tt.exists)
		}
	}
}
