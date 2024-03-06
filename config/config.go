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
	"os"
	"path"

	"github.com/pelletier/go-toml/v2"
)

const (
	DefaultConfigPath = "/etc/habana-container-runtime/config.toml"
	driverPath        = "/run/habana/driver"
	configOverride    = "XDG_CONFIG_HOME"
	configFilePath    = "habana-container-runtime/config.toml"

	hookDefaultFilePath = "/usr/bin/habana-container-hook"
	defaultL3Config     = "/etc/habanalabs/gaudinet.json"
)

const (
	ModeOCI    string = "oci"
	ModeLegacy string = "legacy"
	// TDB
	ModeCDI string = "cdi"
)

var configDir = "/etc/"

type Config struct {
	NetworkL3Config          NetworkConfig `toml:"network-layer-routes"`
	CLI                      CLIConfig     `toml:"habana-container-cli"`
	Runtime                  RuntimeConfig `toml:"habana-container-runtime"`
	AcceptEnvvarUnprivileged bool          `toml:"accept-habana-visible-devices-envvar-when-unprivileged"`
	MountAccelerators        bool          `toml:"mount_accelerators"`
	MountUverbs              bool          `toml:"mount_uverbs"`
	BinariesDir              string        `toml:"binaries-dir"`
}

type NetworkConfig struct {
	Path string `toml:"path"`
}

type RuntimeConfig struct {
	DebugFilePath string     `toml:"debug"`
	Mode          string     `toml:"mode"`
	LogLevel      slog.Level `toml:"log_level"`
	AlwaysMount   bool       `toml:"visible_devices_all_as_default"`
	SystemdCgroup bool       `toml:"systemd_cgroup"`
}

type CLIConfig struct {
	Root        *string  `toml:"root"`
	Path        *string  `toml:"path"`
	Debug       string   `toml:"debug"`
	Environment []string `toml:"environment"`
}

func Load() (*Config, error) {
	cfg := defaultConfig()

	if XDGConfigDir := os.Getenv(configOverride); len(XDGConfigDir) != 0 {
		configDir = XDGConfigDir
	}
	configFilePath := path.Join(configDir, configFilePath)

	f, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	err = toml.NewDecoder(f).Decode(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func defaultConfig() Config {
	return Config{
		MountAccelerators: true,
		MountUverbs:       true,
		BinariesDir:       "/usr/local/bin",
		NetworkL3Config: NetworkConfig{
			Path: defaultL3Config,
		},
		Runtime: RuntimeConfig{
			AlwaysMount:   true,
			DebugFilePath: "/dev/null",
			LogLevel:      slog.LevelInfo,
			SystemdCgroup: false,
			Mode:          ModeOCI,
		},
		CLI: CLIConfig{
			Root:        nil,
			Path:        nil,
			Environment: []string{},
			Debug:       "/dev/null",
		},
	}
}
