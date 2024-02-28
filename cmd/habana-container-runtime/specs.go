package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/HabanaAI/habana-container-runtime/discover"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func loadSpecs(bundleConfigFile string) (*specs.Spec, error) {
	jsonFile, err := os.OpenFile(filepath.Clean(bundleConfigFile), os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening OCI spec file: %w", err)
	}
	defer jsonFile.Close()

	var spec specs.Spec
	err = json.NewDecoder(jsonFile).Decode(&spec)
	if err != nil {
		return nil, fmt.Errorf("reading OCI spec file: %w", err)
	}

	return &spec, nil
}

func saveSpecs(bundleConfigFile string, spec *specs.Spec) error {
	jsonFile, err := os.OpenFile(filepath.Clean(bundleConfigFile), os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening OCI spec file: %w", err)
	}
	defer jsonFile.Close()

	jsonOutput, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling OCI spec: %w", err)
	}

	_, err = jsonFile.WriteAt(jsonOutput, 0)
	if err != nil {
		return fmt.Errorf("writing to OCI spec file: %w", err)
	}
	return nil
}

func addPrestartHook(logger *slog.Logger, spec *specs.Spec) error {
	path, err := execLookPath("habana-container-runtime-hook")
	if err != nil {
		path = hookDefaultFilePath
		_, err = os.Stat(path)
		if err != nil {
			return err
		}
	}
	logger.Info("Prestart hook path", "path", path)

	args := []string{path}
	if spec.Hooks == nil {
		spec.Hooks = &specs.Hooks{}
	} else if len(spec.Hooks.Prestart) != 0 {
		for _, hook := range spec.Hooks.Prestart {
			if !strings.Contains(hook.Path, "habana-container-runtime-hook") {
				continue
			}
			logger.Info("Existing habana prestart hook in OCI spec file")
			return nil
		}
	}

	spec.Hooks.Prestart = append(spec.Hooks.Prestart, specs.Hook{
		Path: path,
		Args: append(args, "prestart"),
	})

	logger.Info("Prestart hook added, executing runc")
	return nil
}

func addCreateRuntimeHook(logger *slog.Logger, spec *specs.Spec) error {
	path, err := execLookPath("habana-container-runtime-hook")
	if err != nil {
		path = hookDefaultFilePath
		_, err = os.Stat(path)
		if err != nil {
			return err
		}
	}
	logger.Info("hook binary path", "path", path)

	args := []string{path}
	if spec.Hooks == nil {
		spec.Hooks = &specs.Hooks{}
	} else if len(spec.Hooks.CreateRuntime) != 0 {
		for _, hook := range spec.Hooks.CreateRuntime {
			if !strings.Contains(hook.Path, "habana-container-runtime-hook") {
				continue
			}
			logger.Info("Existing habana createRuntime hook in OCI spec file")
			return nil
		}
	}

	spec.Hooks.CreateRuntime = append(spec.Hooks.CreateRuntime, specs.Hook{
		Path: path,
		Args: append(args, "createRuntime"),
	})

	logger.Info("createRuntime hook added, executing runc")
	return nil
}

func addAcceleratorDevices(logger *slog.Logger, spec *specs.Spec, requestedDevs []string) error {
	logger.Debug("Discovering accelerators")

	// Extract module id for HABANA_VISIBLE_MODULES environment variables
	modulesIDs := make([]string, 0, len(requestedDevs))
	for _, acc := range requestedDevs {
		id, err := discover.AcceleratorModuleID(acc)
		if err != nil {
			logger.Debug("discoring modules")
			return err
		}
		modulesIDs = append(modulesIDs, id)
	}

	// Prepare devices in OCI format
	var devs []*discover.DevInfo
	for _, u := range requestedDevs {
		p := fmt.Sprintf("/dev/accel/accel%s", u)
		logger.Info("Adding accelerator device", "path", p)
		i, err := discover.DeviceInfo(p)
		if err != nil {
			return err
		}
		devs = append(devs, i)
	}

	addDevicesToSpec(logger, spec, devs)
	addAllowList(logger, spec, devs)
	addEnvVar(spec, EnvHLVisibleModules, strings.Join(modulesIDs, ","))

	return nil
}

func addUverbsDevices(logger *slog.Logger, spec *specs.Spec, requestedDevs []string) error {
	logger.Debug("Discovering uverbs")

	var devs []*discover.DevInfo
	for _, v := range requestedDevs {
		hlib := fmt.Sprintf("/sys/class/infiniband/hlib_%s", v)
		logger.Debug("Getting uverbs device for hlib", "hlib", hlib)

		// Extract uverb from hlib device
		uverbs, err := osReadDir(fmt.Sprintf("%s/device/infiniband_verbs", hlib))
		if err != nil {
			logger.Error(fmt.Sprintf("Reading hlib directory: %v", err))
			continue
		}
		if len(uverbs) == 0 {
			logger.Debug("No uverbs devices found for devices", "device", hlib)
			continue
		}
		uverbDev := fmt.Sprintf("/dev/infiniband/%s", uverbs[0].Name())

		// Prepare devices in OCI format
		logger.Info("Adding uverb device", "path", uverbDev)
		i, err := discover.DeviceInfo(uverbDev)
		if err != nil {
			return err
		}
		logger.Info("Adding uverb device", "path", uverbDev)
		devs = append(devs, i)
	}

	addDevicesToSpec(logger, spec, devs)
	addAllowList(logger, spec, devs)

	return nil
}

func filterDevicesByENV(spec *specs.Spec, devices []string) []string {
	var requestedDevs []string
	for _, ev := range spec.Process.Env {
		if strings.HasPrefix(ev, "HABANA_VISIBLE_DEVICES") {
			_, values, found := strings.Cut(ev, "=")
			if found {
				if values == "all" {
					return devices
				} else {
					requestedDevs = strings.Split(values, ",")
				}
			}
			break
		}
	}

	// Case when alwaysMatch is true, and user didn't provide the environment variable
	if len(requestedDevs) == 0 {
		return devices
	}

	var filteredDevices []string
	for _, dev := range devices {
		devID := string(dev[len(dev)-1])
		if slices.Contains(requestedDevs, devID) {
			filteredDevices = append(filteredDevices, dev)
		}
	}

	return filteredDevices
}

// addDevicesToSpec adds list of devices nodes to be created for container.
func addDevicesToSpec(logger *slog.Logger, spec *specs.Spec, devices []*discover.DevInfo) {
	logger.Debug("Mounting devices in spec")
	current := make(map[string]struct{})

	for _, dev := range spec.Linux.Devices {
		current[dev.Path] = struct{}{}
	}

	var devicesToAdd []specs.LinuxDevice
	for _, hlDevice := range devices {
		if _, ok := current[hlDevice.Path]; ok {
			continue
		}

		zeroID := uint32(0)
		devicesToAdd = append(devicesToAdd, specs.LinuxDevice{
			Type:     "c",
			Major:    int64(hlDevice.Major),
			Minor:    int64(hlDevice.Minor),
			FileMode: &hlDevice.FileMode,
			Path:     hlDevice.Path,
			GID:      &zeroID,
			UID:      &zeroID,
		})
		logger.Debug("Added device to spec", "path", hlDevice.Path)
	}

	spec.Linux.Devices = append(spec.Linux.Devices, devicesToAdd...)
}

// addAllowList modifies the Linux devices allow list to cgroup rules.
func addAllowList(logger *slog.Logger, spec *specs.Spec, devices []*discover.DevInfo) {
	logger.Debug("Adding devices to allow list")

	current := make(map[string]bool)
	for _, dev := range spec.Linux.Resources.Devices {
		if dev.Major != nil && dev.Minor != nil {
			current[fmt.Sprintf("%d-%d", *dev.Major, *dev.Minor)] = true
		}
	}

	var devsToAdd []specs.LinuxDeviceCgroup
	for _, hldev := range devices {
		k := fmt.Sprintf("%d-%d", hldev.Major, hldev.Minor)
		if _, ok := current[k]; !ok {
			major := int64(hldev.Major)
			minor := int64(hldev.Minor)
			devsToAdd = append(devsToAdd, specs.LinuxDeviceCgroup{
				Allow:  true,
				Type:   "c",
				Major:  &major,
				Minor:  &minor,
				Access: "rwm",
			})
			logger.Debug("Added device to allow list", "major", hldev.Major, "minor", hldev.Minor)
		}
	}

	// modify spec
	spec.Linux.Resources.Devices = append(spec.Linux.Resources.Devices, devsToAdd...)
}

func addEnvVar(spec *specs.Spec, key string, value string) {
	spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("%s=%v", key, strconv.Quote(value)))
}
