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
