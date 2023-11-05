package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestRun(t *testing.T) {
	execRunc = func(logger *slog.Logger, args []string) error {
		return nil
	}

	// t.Run("modified spec for create command", func(t *testing.T) {
	// 	createArgs := `--root /run/containerd/runc/k8s.io --log /run/containerd/io.containerd.runtime.v2.task/k8s.io/258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88/log.json --log-format json create --bundle ./testdata/input --pid-file /run/containerd/io.containerd.runtime.v2.task/k8s.io/258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88/init.pid 258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88`

	// })

	// t.Run("leaves the spec without any changes", func(t *testing.T) {
	// 	nonCreateArgs := `--root /run/containerd/runc/k8s.io --log /run/containerd/io.containerd.runtime.v2.task/k8s.io/258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88/log.json --log-format json delete --force 258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88`

	// })
}

func TestGetConfig(t *testing.T) {
	configDir = "testdata/input"
	cfg, err := getConfig()
	if err != nil {
		t.Fatalf("getConfig() err=%q, want nil", err)
	}
	want := "/tmp/runtime-test"
	if cfg.debugFilePath != want {
		t.Errorf("got %q, want %q", cfg.debugFilePath, want)
	}
}

func TestModifySpec(t *testing.T) {
	// Prepare
	execLookPath = func(file string) (string, error) {
		return "/usr/bin/habana-container-runtime-hook", nil
	}

	tmpDir, tmpFile := prepareTempFile(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))
	err := modifySpec(logger, tmpDir)
	if err != nil {
		t.Fatalf("modifySpec() err=%q, want nil", err)
	}

	// Get expected output for comparison
	want := fileToSpecs(t, "testdata/output/for_create.json")
	got := fileToSpecs(t, tmpFile)

	if !cmp.Equal(got.Hooks, want.Hooks) {
		t.Errorf(cmp.Diff(got.Hooks, want.Hooks))
	}
}

func prepareTempFile(t *testing.T) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile, err := os.Create(filepath.Clean(tmpDir) + "/config.json")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()

	t.Cleanup(func() {
		_ = os.Remove(tmpFile.Name())
	})

	// Copy golden input to temporary file
	gf, err := os.Open("testdata/input/config.json")
	if err != nil {
		t.Fatal(err)
	}
	defer gf.Close()

	_, err = io.Copy(tmpFile, gf)
	if err != nil {
		t.Fatal(err)
	}
	return tmpDir, tmpFile.Name()
}

func fileToSpecs(t *testing.T, filePath string) specs.Spec {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		t.Fatal(err)
	}

	var spec specs.Spec
	err = json.Unmarshal(data, &spec)
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func TestHasCreateCommand(t *testing.T) {
	createArgs := `--root /run/containerd/runc/k8s.io --log /run/containerd/io.containerd.runtime.v2.task/k8s.io/258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88/log.json --log-format json create --bundle ./testdata/input --pid-file /run/containerd/io.containerd.runtime.v2.task/k8s.io/258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88/init.pid 258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88`
	deleteArgs := `--root /run/containerd/runc/k8s.io --log /run/containerd/io.containerd.runtime.v2.task/k8s.io/258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88/log.json --log-format json delete --force 258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88`
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{
			name: "with create command",
			in:   createArgs,
			want: true,
		},
		{
			name: "with delete command",
			in:   deleteArgs,
			want: false,
		},
		{
			name: "non meaningfull words",
			in:   "--bla --create -create test foo bar",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := hasCreateCommand(strings.Fields(tt.in)); got != tt.want {
				t.Errorf("want %t, got %t", got, tt.want)
			}
		})
	}

}
