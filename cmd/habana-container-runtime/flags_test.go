package main

import (
	"os"
	"strings"
	"testing"
)

func TestParseBundle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "with full bundle flag",
			input: "--root true --bundle /var/run/containerd/bundle create another",
			want:  "/var/run/containerd/bundle",
		},
		{
			name:  "with short bundle flag",
			input: "--root true -b /var/run/containerd/bundle create another",
			want:  "/var/run/containerd/bundle",
		},
		{
			name:  "with equal sign separator",
			input: "--root true --bundle=/var/run/containerd/bundle create another",
			want:  "/var/run/containerd/bundle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBundle(strings.Fields(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseBundleWithoutFlag(t *testing.T) {
	want, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseBundle([]string{"nothing here"})
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
