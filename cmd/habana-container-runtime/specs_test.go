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
	"fmt"
	"reflect"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestFilterDevicesByENV(t *testing.T) {
	tests := []struct {
		name       string
		spec       specs.Spec
		devices    []string
		expDevices []string
	}{
		{
			name: "no env var return all devices",
			spec: specs.Spec{
				Process: &specs.Process{
					Env: []string{},
				},
			},
			devices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
				"/dev/accel/accel1",
				"/dev/accel/accel_controlD1",
				"/dev/infiniband/uverbs1",
			},
			expDevices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
				"/dev/accel/accel1",
				"/dev/accel/accel_controlD1",
				"/dev/infiniband/uverbs1",
			},
		},
		{
			name: "env var without values returns all devices",
			spec: specs.Spec{
				Process: &specs.Process{
					Env: []string{
						EnvHLVisibleDevices,
					},
				},
			},
			devices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
				"/dev/accel/accel1",
				"/dev/accel/accel_controlD1",
				"/dev/infiniband/uverbs1",
			},
			expDevices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
				"/dev/accel/accel1",
				"/dev/accel/accel_controlD1",
				"/dev/infiniband/uverbs1",
			},
		},
		{
			name: "env var with 'all' returns all devices",
			spec: specs.Spec{
				Process: &specs.Process{
					Env: []string{
						fmt.Sprintf("%s=all", EnvHLVisibleDevices),
					},
				},
			},
			devices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
				"/dev/accel/accel1",
				"/dev/accel/accel_controlD1",
				"/dev/infiniband/uverbs1",
			},
			expDevices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
				"/dev/accel/accel1",
				"/dev/accel/accel_controlD1",
				"/dev/infiniband/uverbs1",
			},
		},
		{
			name: "env var with single value returns only requested device",
			spec: specs.Spec{
				Process: &specs.Process{
					Env: []string{
						fmt.Sprintf("%s=0", EnvHLVisibleDevices),
					},
				},
			},
			devices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
				"/dev/accel/accel1",
				"/dev/accel/accel_controlD1",
				"/dev/infiniband/uverbs1",
			},
			expDevices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
			},
		},
		{
			name: "env var with multiple values returns only requested device",
			spec: specs.Spec{
				Process: &specs.Process{
					Env: []string{
						fmt.Sprintf("%s,=0,1,2", EnvHLVisibleDevices),
					},
				},
			},
			devices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
				"/dev/accel/accel1",
				"/dev/accel/accel_controlD1",
				"/dev/accel/accel2",
				"/dev/accel/accel_controlD2",
				"/dev/accel/accel3",
				"/dev/accel/accel_controlD3",
			},
			expDevices: []string{
				"/dev/accel/accel0",
				"/dev/accel/accel_controlD0",
				"/dev/accel/accel1",
				"/dev/accel/accel_controlD1",
				"/dev/accel/accel2",
				"/dev/accel/accel_controlD2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.spec
			got := filterDevicesByENV(&s, tt.devices)
			if len(got) != len(tt.expDevices) {
				t.Errorf("got=%d devices, want %d devices", len(got), len(tt.expDevices))
			}
			if !reflect.DeepEqual(got, tt.expDevices) {
				t.Errorf("got=%v, want %v", got, tt.expDevices)
			}
		})
	}
}
