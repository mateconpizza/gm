package sys

import (
	"log/slog"
	"os"
	"testing"
)

//nolint:paralleltest //test
func TestEnv(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "EnvSet",
			key:   "TEST_KEY",
			value: "testValue",
		},
		{
			name:  "EnvNotSet",
			key:   "TEST_KEY",
			value: "",
		},
		{
			name:  "EnvOne",
			key:   "TEST_KEY",
			value: "1",
		},
	}

	for _, tt := range tests {
		t.Setenv(tt.key, tt.value)
		got := Env(tt.key, tt.value)

		// unset env after obtaining value
		if err := os.Unsetenv(tt.key); err != nil {
			slog.Error("unsetting env", "env", tt.key, "error", err)
		}

		if got != tt.value {
			t.Errorf("%s: got: %v, expected: %s", tt.name, got, tt.value)
		}
	}
}

func TestBinExists(t *testing.T) {
	t.Parallel()
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
		err := Which(tt.command)
		if err != nil && tt.exists {
			t.Errorf("%s: got: %v, expected: nil", tt.command, err)
		}
		if err == nil && !tt.exists {
			t.Errorf("%s: got: nil, expected: error", tt.command)
		}
	}
}
