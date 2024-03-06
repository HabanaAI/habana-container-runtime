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
	"log"
	"os"
	"path"

	"github.com/BurntSushi/toml"
)

const (
	configPath = "/etc/habana-container-runtime/config.toml"
	driverPath = "/run/habana/driver"
)

var defaultPaths = [...]string{
	path.Join(driverPath, configPath),
	configPath,
}

// CLIConfig : options for habana-container-cli.
type CLIConfig struct {
	Root        *string  `toml:"root"`
	Path        *string  `toml:"path"`
	Environment []string `toml:"environment"`
	Debug       *string  `toml:"debug"`
	// Mount accelerator devices from the container-runtime. If running in kubernetes
	// environment with Habana Device Plugin, can leave it undefined or set to false,
	// since device plugin will mount the accelerator devices.
	MountAccelerators *bool `toml:"mount_accelerators"`
	// Mount infiniband verbs devices
	MountUverbs *bool `toml:"mount_uverbs"`
}

// HookConfig : options for the habana-container-hook.
type HookConfig struct {
	AcceptEnvvarUnprivileged bool `toml:"accept-habana-visible-devices-envvar-when-unprivileged"`

	HabanaContainerCLI CLIConfig `toml:"habana-container-cli"`
}

func getDefaultHookConfig() (config HookConfig) {
	return HookConfig{
		AcceptEnvvarUnprivileged: true,
		HabanaContainerCLI: CLIConfig{
			Root:        nil,
			Path:        nil,
			Environment: []string{},
			Debug:       nil,
		},
	}
}

func getHookConfig() (config HookConfig) {
	var err error

	if len(*configflag) > 0 {
		config = getDefaultHookConfig()
		_, err = toml.DecodeFile(*configflag, &config)
		if err != nil {
			log.Panicln("couldn't open configuration file:", err)
		}
	} else {
		for _, p := range defaultPaths {
			config = getDefaultHookConfig()
			_, err = toml.DecodeFile(p, &config)
			if err == nil {
				break
			} else if !os.IsNotExist(err) {
				log.Panicln("couldn't open default configuration file:", err)
			}
		}
	}

	return config
}
