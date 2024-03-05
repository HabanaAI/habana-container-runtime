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
	"encoding/json"
	"reflect"
	"testing"
)

func TestGetHabanaConfig(t *testing.T) {
	tests := []struct {
		description    string
		env            map[string]string
		privileged     bool
		expectedConfig *habanaConfig
		expectedPanic  bool
	}{
		{
			description:    "No environment, unprivileged",
			env:            map[string]string{},
			privileged:     false,
			expectedConfig: nil,
		},
		{
			description:    "No environment, privileged",
			env:            map[string]string{},
			privileged:     true,
			expectedConfig: nil,
		},
		{
			description: "environment 'all', privileged",
			env: map[string]string{
				envHBVisibleDevices: "all",
			},
			privileged: true,
			expectedConfig: &habanaConfig{
				Devices: "all",
			},
		},
		{
			description: "environment 'all', unprivileged",
			env: map[string]string{
				envHBVisibleDevices: "all",
			},
			privileged: false,
			expectedConfig: &habanaConfig{
				Devices: "all",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var config *habanaConfig
			getConfig := func() {
				hookConfig := getDefaultHookConfig()
				config = getHabanaConfig(&hookConfig, tc.env, nil, tc.privileged)
			}
			if tc.expectedPanic {
				// panic
			}

			getConfig()

			if config == nil && tc.expectedConfig == nil {
				return
			}
			if config != nil && tc.expectedConfig != nil {
				if !reflect.DeepEqual(config.Devices, tc.expectedConfig.Devices) {
					t.Errorf("Unexpected habanaConfig (got: %v, wanted: %v)", config, tc.expectedConfig)
				}
				return
			}
			t.Errorf("Unexpected habanaConfig (got: %v, wanted %v)", config, tc.expectedConfig)
		})
	}
}

func TestDeviceListSourcePriority(t *testing.T) {
	tests := []struct {
		description        string
		envvarDevices      string
		privileged         bool
		acceptUnprivileged bool
		expectedDevices    *string
	}{
		{
			description:        "No mount devices, unrivileged, accept unprivileged",
			envvarDevices:      "0,1",
			privileged:         false,
			acceptUnprivileged: true,
			expectedDevices:    &[]string{"0,1"}[0],
		},
		{
			description:        "No mount devices, privileged, accept unprivileged",
			envvarDevices:      "0,1",
			privileged:         true,
			acceptUnprivileged: true,
			expectedDevices:    &[]string{"0,1"}[0],
		},
		{
			description:        "No mount devices, unrivileged, accept unprivileged",
			envvarDevices:      "all",
			privileged:         false,
			acceptUnprivileged: true,
			expectedDevices:    &[]string{"all"}[0],
		},
		{
			description:        "No mount devices, privileged, accept unprivileged",
			envvarDevices:      "all",
			privileged:         true,
			acceptUnprivileged: true,
			expectedDevices:    &[]string{"all"}[0],
		},
		{
			description:        "no devices",
			envvarDevices:      "",
			privileged:         true,
			acceptUnprivileged: true,
			expectedDevices:    nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			// Wrap the call to getDevices() in a closure.
			var devices *string
			getDevices := func() {
				env := map[string]string{
					envHBVisibleDevices: tc.envvarDevices,
				}
				hookConfig := getDefaultHookConfig()
				hookConfig.AcceptEnvvarUnprivileged = tc.acceptUnprivileged
				devices = getDevices(&hookConfig, env, []Mount{}, tc.privileged, false)
			}

			// For all other tests, just grab the devices and check the results
			getDevices()
			if !reflect.DeepEqual(devices, tc.expectedDevices) {
				if tc.expectedDevices != nil {
					t.Errorf("Unexpected devices (got: %v, wanted: %v)", *devices, *tc.expectedDevices)
				}
			}
		})
	}
}

func TestIsPrivileged(t *testing.T) {
	tests := []struct {
		spec     string
		expected bool
	}{
		{
			`
			{
				"ociVersion": "1.0.0",
				"process": {
					"capabilities": {
						"bounding": [ "CAP_SYS_ADMIN" ]
					}
				}
			}
			`,
			true,
		},
		{
			`
			{
				"ociVersion": "1.0.0",
				"process": {
					"capabilities": {
						"bounding": [ "CAP_SYS_OTHER" ]
					}
				}
			}
			`,
			false,
		},
		{
			`
			{
				"ociVersion": "1.0.0",
				"process": {}
			}
			`,
			false,
		},
		{
			`
			{
				"ociVersion": "1.0.0-rc2-dev",
				"process": {
					"capabilities": [ "CAP_SYS_ADMIN" ]
				}
			}
			`,
			true,
		},
		{
			`
			{
				"ociVersion": "1.0.0-rc2-dev",
				"process": {
					"capabilities": [ "CAP_SYS_OTHER" ]
				}
			}
			`,
			false,
		},
		{
			`
			{
				"ociVersion": "1.0.0-rc2-dev",
				"process": {}
			}
			`,
			false,
		},
	}
	for _, tc := range tests {
		var spec Spec
		_ = json.Unmarshal([]byte(tc.spec), &spec)
		privileged := isPrivileged(&spec)
		if privileged != tc.expected {
			t.Errorf("isPrivileged() returned unexpectred value (privileged: %v, tc.expected: %v)", privileged, tc.expected)
		}
	}
}
