package netinfo

import (
	"errors"
	"testing"
)

func TestDeviceType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		rfFunc   func(name string) ([]byte, error)
		want     string
		expError bool
	}{
		{
			name:  "happy path",
			input: "0",
			rfFunc: func(name string) ([]byte, error) {
				return []byte("GAUDI2"), nil
			},
			want:     "gaudi2",
			expError: false,
		},
		{
			name:  "file contains new line",
			input: "0",
			rfFunc: func(name string) ([]byte, error) {
				return []byte("GAUDI2\n"), nil
			},
			want:     "gaudi2",
			expError: false,
		},
		{
			name:  "file contains new line and space char",
			input: "0",
			rfFunc: func(name string) ([]byte, error) {
				return []byte(" GAUDI2\t\n"), nil
			},
			want:     "gaudi2",
			expError: false,
		},
		{
			name:  "empty file returns an error",
			input: "0",
			rfFunc: func(name string) ([]byte, error) {
				return []byte(""), nil
			},
			want:     "",
			expError: true,
		},
		{
			name:  "failed to open file",
			input: "0",
			rfFunc: func(name string) ([]byte, error) {
				return nil, errors.New("foo")
			},
			want:     "",
			expError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osReadFile = tt.rfFunc

			got, err := deviceType(tt.input)
			if tt.expError && err == nil {
				t.Fatal("expected and error, got none")
			}
			if !tt.expError && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpPortsMACAddresses(t *testing.T) {
}
