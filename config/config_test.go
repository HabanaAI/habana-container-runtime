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
		Runtime: RuntimeConfig{
			AlwaysMount:   false,
			DebugFilePath: "/tmp/runtime-test",
			LogLevel:      slog.LevelDebug,
			SystemdCgroup: true,
			Mode:          ModeLegacy,
		},
		NetworkL3Config: NetworkConfig{
			"/tmp/testdata.json",
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v\nwant %+v", cfg, want)
	}
}
