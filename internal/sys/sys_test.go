package sys

import (
	"log"
	"os"
	"testing"
)

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
