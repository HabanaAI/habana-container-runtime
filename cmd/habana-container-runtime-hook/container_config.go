/*
 * Copyright (c) 2021, HabanaLabs Ltd.  All rights reserved.
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
	"encoding/json"
	"log"
	"os"
	"path"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	envHBVisibleDevices = "HABANA_VISIBLE_DEVICES"
)

const (
	capSysAdmin = "CAP_SYS_ADMIN"
)

type habanaConfig struct {
	Devices string
}

type containerConfig struct {
	Pid    int
	Rootfs string
	Env    map[string]string
	Habana *habanaConfig
}

// Root from OCI runtime spec
// github.com/opencontainers/runtime-spec/blob/v1.0.0/specs-go/config.go#L94-L100
type Root struct {
	Path string `json:"path"`
}

// Process from OCI runtime spec
// github.com/opencontainers/runtime-spec/blob/v1.0.0/specs-go/config.go#L30-L57
type Process struct {
	Env          []string         `json:"env,omitempty"`
	Capabilities *json.RawMessage `json:"capabilities,omitempty" platform:"linux"`
}

// LinuxCapabilities from OCI runtime spec
// https://github.com/opencontainers/runtime-spec/blob/v1.0.0/specs-go/config.go#L61
type LinuxCapabilities struct {
	Bounding    []string `json:"bounding,omitempty" platform:"linux"`
	Effective   []string `json:"effective,omitempty" platform:"linux"`
	Inheritable []string `json:"inheritable,omitempty" platform:"linux"`
	Permitted   []string `json:"permitted,omitempty" platform:"linux"`
	Ambient     []string `json:"ambient,omitempty" platform:"linux"`
}

// Mount from OCI runtime spec
// https://github.com/opencontainers/runtime-spec/blob/v1.0.0/specs-go/config.go#L103
type Mount struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type,omitempty" platform:"linux,solaris"`
	Source      string   `json:"source,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// Spec from OCI runtime spec
// We use pointers to structs, similarly to the latest version of runtime-spec:
// https://github.com/opencontainers/runtime-spec/blob/v1.0.0/specs-go/config.go#L5-L28
type Spec struct {
	Version *string  `json:"ociVersion"`
	Process *Process `json:"process,omitempty"`
	Root    *Root    `json:"root,omitempty"`
	Mounts  []Mount  `json:"mounts,omitempty"`
}

// HookState holds state information about the hook
type HookState struct {
	Pid int `json:"pid,omitempty"`
	// After 17.06, runc is using the runtime spec:
	// github.com/docker/runc/blob/17.06/libcontainer/configs/config.go#L262-L263
	// github.com/opencontainers/runtime-spec/blob/v1.0.0/specs-go/state.go#L3-L17
	Bundle string `json:"bundle"`
	// Before 17.06, runc used a custom struct that didn't conform to the spec:
	// github.com/docker/runc/blob/17.03.x/libcontainer/configs/config.go#L245-L252
	BundlePath string `json:"bundlePath"`
}

func getEnvMap(e []string) (m map[string]string) {
	m = make(map[string]string)
	for _, s := range e {
		p := strings.SplitN(s, "=", 2)
		if len(p) != 2 {
			log.Panicln("environment error")
		}
		m[p[0]] = p[1]
	}
	return
}

func loadSpec(path string) (spec *Spec) {
	f, err := os.Open(path)
	if err != nil {
		log.Panicln("could not open OCI spec:", err)
	}
	defer f.Close()

	if err = json.NewDecoder(f).Decode(&spec); err != nil {
		log.Panicln("could not decode OCI spec:", err)
	}
	if spec.Version == nil {
		log.Panicln("Version is empty in OCI spec")
	}
	if spec.Process == nil {
		log.Panicln("Process is empty in OCI spec")
	}
	if spec.Root == nil {
		log.Panicln("Root is empty in OCI spec")
	}
	return
}

func isPrivileged(s *Spec) bool {
	if s.Process.Capabilities == nil {
		return false
	}

	var caps []string
	// If v1.1.0-rc1 <= OCI version < v1.0.0-rc5 parse s.Process.Capabilities as:
	// github.com/opencontainers/runtime-spec/blob/v1.0.0-rc1/specs-go/config.go#L30-L54
	rc1cmp := semver.Compare("v"+*s.Version, "v1.0.0-rc1")
	rc5cmp := semver.Compare("v"+*s.Version, "v1.0.0-rc5")
	if (rc1cmp == 1 || rc1cmp == 0) && (rc5cmp == -1) {
		err := json.Unmarshal(*s.Process.Capabilities, &caps)
		if err != nil {
			log.Panicln("could not decode Process.Capabilities in OCI spec:", err)
		}
		// Otherwise, parse s.Process.Capabilities as:
		// github.com/opencontainers/runtime-spec/blob/v1.0.0/specs-go/config.go#L30-L54
	} else {
		var lc LinuxCapabilities
		err := json.Unmarshal(*s.Process.Capabilities, &lc)
		if err != nil {
			log.Panicln("could not decode Process.Capabilities in OCI spec:", err)
		}
		// We only make sure that the bounding capabibility set has
		// CAP_SYS_ADMIN. This allows us to make sure that the container was
		// actually started as '--privileged', but also allow non-root users to
		// access the privileged HABANA capabilities.
		caps = lc.Bounding
	}

	for _, c := range caps {
		if c == capSysAdmin {
			return true
		}
	}

	return false
}

func getDevicesFromEnvvar(env map[string]string, legacyImage bool) *string {
	// Build a list of envvars to consider.
	envVars := []string{envHBVisibleDevices}

	// Grab a reference to devices from the first envvar
	// in the list that actually exists in the environment.
	var devices *string
	for _, envVar := range envVars {
		if devs, ok := env[envVar]; ok {
			devices = &devs
		}
	}

	// Environment variable unset with legacy image: default to "all".
	if devices == nil && legacyImage {
		all := "all"
		return &all
	}

	// Environment variable unset or empty or "void": return nil
	if devices == nil || len(*devices) == 0 || *devices == "void" {
		all := "all"
		return &all
	}

	// Environment variable set to "none": reset to "".
	if *devices == "none" {
		empty := ""
		return &empty
	}

	// Any other value.
	return devices
}

func getDevices(hookConfig *HookConfig, env map[string]string, mounts []Mount, privileged bool, legacyImage bool) *string {
	// Fallback to reading from the environment variable if privileges are correct
	devices := getDevicesFromEnvvar(env, legacyImage)
	if devices == nil {
		return nil
	}
	if privileged || hookConfig.AcceptEnvvarUnprivileged {
		return devices
	}

	// Error out otherwise
	log.Panicln("insufficient privileges to read device list from HABANA_VISIBLE_DEVICES envvar")

	return nil
}

func getHabanaConfig(hookConfig *HookConfig, env map[string]string, mounts []Mount, privileged bool) *habanaConfig {
	legacyImage := false

	var devices string
	if d := getDevices(hookConfig, env, mounts, privileged, legacyImage); d != nil {
		devices = *d
	} else {
		// 'nil' devices means this is not a HL container.
		return nil
	}

	return &habanaConfig{
		Devices: devices,
	}
}

func getContainerConfig(hook HookConfig) (config containerConfig) {
	var h HookState
	d := json.NewDecoder(os.Stdin)
	if err := d.Decode(&h); err != nil {
		log.Panicln("could not decode container state:", err)
	}

	b := h.Bundle
	if len(b) == 0 {
		b = h.BundlePath
	}

	s := loadSpec(path.Join(b, "config.json"))

	env := getEnvMap(s.Process.Env)
	privileged := isPrivileged(s)
	return containerConfig{
		Pid:    h.Pid,
		Rootfs: s.Root.Path,
		Env:    env,
		Habana: getHabanaConfig(&hook, env, s.Mounts, privileged),
	}
}
