/*
 * Copyright (c) 2022, HabanaLabs Ltd.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
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
