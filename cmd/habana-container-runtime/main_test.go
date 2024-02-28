package main

import (
	"strings"
	"testing"
)

func TestHasCreateCommand(t *testing.T) {
	t.Parallel()

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
			name: "with create flags but not command",
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
