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
package config

import (
	"log/slog"
	"reflect"
	"testing"
)

func TestGetConfig(t *testing.T) {
	configDir = "testdata/input"
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() err=%q, want nil", err)
	}
	want := &Config{
		MountAccelerators: true,
		MountUverbs:       true,
		BinariesDir:       "/usr/local/bin",
		Runtime: RuntimeConfig{
			AlwaysMount:   false,
			DebugFilePath: "/tmp/runtime-test",
			LogLevel:      slog.LevelDebug,
			SystemdCgroup: true,
			Mode:          ModeLegacy,
		},
		CLI: CLIConfig{
			Debug:       "/dev/null",
			Root:        nil,
			Path:        nil,
			Environment: []string{},
		},
		NetworkL3Config: NetworkConfig{
			"/tmp/testdata.json",
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v\nwant %+v", cfg, want)
	}
}
