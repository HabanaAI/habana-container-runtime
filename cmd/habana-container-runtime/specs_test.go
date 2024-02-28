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
