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
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"

	"github.com/HabanaAI/habana-container-runtime/config"
	"github.com/HabanaAI/habana-container-runtime/discover"
	"github.com/HabanaAI/habana-container-runtime/netinfo"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	hookDefaultFilePath = "/usr/bin/habana-container-hook"
	defaultL3Config     = "/etc/habanalabs/gaudinet.json"
)

const (
	EnvHLVisibleDevices = "HABANA_VISIBLE_DEVICES"
	EnvHLVisibleModules = "HABANA_VISIBLE_MODULES"
	EnvHLRuntimeError   = "HABANA_RUNTIME_ERROR"
)

var (
	osReadDir    = os.ReadDir
	execRunc     = execRuncFunc
	execLookPath = exec.LookPath
	osStat       = os.Stat
	osExecutable = os.Executable
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(cfg.Runtime.DebugFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger := slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: cfg.Runtime.LogLevel}))

	if err := run(logger, cfg, os.Args[1:]); err != nil {
		logger.Error(err.Error())
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
}

func run(logger *slog.Logger, cfg *config.Config, args []string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
			return
		}
	}()

	err = handleRequest(logger, cfg, args)
	if err != nil {
		return err
	}

	return execRunc(logger, args, cfg.Runtime.SystemdCgroup)
}

// handleRequest manages the flow of the incoming command. Based on the command type
// and container environment variable, we either skip everything altogether, or
// modify the container specs based on the provided environment variables.
func handleRequest(logger *slog.Logger, cfg *config.Config, args []string) error {
	// If has 'create' command', then need to modify runc
	// otherwide, no modification needed
	if !hasCreateCommand(args) {
		logger.Debug("Not a create command, skipping", "command", args)
		return nil
	}

	bundleDir, err := parseBundle(args)
	if err != nil {
		return fmt.Errorf("parsing bundle: %w", err)
	}
	logger.Debug("Bundle directory", "path", bundleDir)

	// Load specs
	bundleConfigFile := bundleDir + "/config.json"
	logger.Info("Using bundle file", "path", bundleConfigFile)

	specConfig, err := loadSpecs(bundleConfigFile)
	if err != nil {
		return fmt.Errorf("loading specs: %w", err)
	}
	defer func() {
		err = saveSpecs(bundleConfigFile, specConfig)
		if err != nil {
			logger.Error(err.Error())
		}
	}()

	// If user didn't ask specifically for always trying to mount the devices
	// to each container, skip. This keeps the environment and runtime flow cleaner,
	// and skips containers that do not asked for devices.
	if !cfg.Runtime.AlwaysMount && !IsHabanaContainer(specConfig) {
		return nil
	}

	// If legacy mode, add habana-hook as a prestart hook, and return to
	// execute runc. The hook and libhabana takes cares of the devices mounts.
	if cfg.Runtime.Mode == config.ModeLegacy {
		logger.Info("In legacy mode")
		err = addPrestartHook(logger, specConfig, cfg)
		if err != nil {
			return fmt.Errorf("adding habana prestart hook: %s", err)
		}
		return nil
	}

	// Always add this hook to expose network interfaces information
	// inside the container
	err = addCreateRuntimeHook(logger, specConfig, cfg)
	if err != nil {
		return fmt.Errorf("adding createRuntime hook: %w", err)
	}

	// We get the available devices based on the user request. If requested device is not
	// available, we'll return here and log the info. If the options is 'all' or not set,
	// we get all the devices.
	requestedDevices := discover.DevicesIDs(filterDevicesByENV(specConfig, discover.AcceleratorDevices()))
	if len(requestedDevices) == 0 {
		logger.Info("No habanalabs accelerators found")
		return nil
	}
	logger.Debug("Requested devices", "devices", requestedDevices)

	if cfg.MountAccelerators {
		err = addAcceleratorDevices(logger, specConfig, requestedDevices)
		if err != nil {
			addErrorEnvVar(specConfig, err.Error())
			return fmt.Errorf("adding accelerator devices: %w", err)
		}
	}

	if cfg.MountUverbs {
		err = addUverbsDevices(logger, specConfig, requestedDevices)
		if err != nil {
			addErrorEnvVar(specConfig, err.Error())
			return fmt.Errorf("adding uverb devices: %w", err)
		}
	}

	// Docker saves the abolute path while containerd mentions the folder name
	// relative to the bundle dir.
	containerRootFS := path.Join(bundleDir, specConfig.Root.Path)
	if path.IsAbs(specConfig.Root.Path) {
		containerRootFS = specConfig.Root.Path
	}

	err = netinfo.Generate(requestedDevices, containerRootFS)
	if err != nil {
		addErrorEnvVar(specConfig, err.Error())
		logger.Error(fmt.Sprintf("generating macAddrInfo failed: %v", err))
	}

	err = netinfo.GaudinetFile(logger, containerRootFS, cfg.NetworkL3Config.Path)
	if err != nil {
		addErrorEnvVar(specConfig, err.Error())
		logger.Error(fmt.Sprintf("generating gaudinet file failed: %v", err))
	}

	return nil
}

func execRuncFunc(logger *slog.Logger, args []string, systemdCgroup bool) error {
	logger.Debug("Looking for 'docker-runc' in PATH")
	runcPath, err := exec.LookPath("docker-runc")
	if err != nil {
		runcPath, err = exec.LookPath("runc")
		if err != nil {
			return err
		}
	}
	logger.Debug("runc path", "path", runcPath)
	cmdArgs := []string{runcPath}
	if systemdCgroup {
		cmdArgs = append(cmdArgs, "--systemd-cgroup")
	}
	cmdArgs = append(cmdArgs, args...)
	logger.Debug("Executing runc command", "cmd", cmdArgs)

	return syscall.Exec(runcPath, cmdArgs, os.Environ())
}

func hasCreateCommand(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if arg == "create" {
			return true
		}
	}
	return false
}

func IsHabanaContainer(spec *specs.Spec) bool {
	for _, ev := range spec.Process.Env {
		if strings.HasPrefix(ev, EnvHLVisibleDevices) {
			return true
		}
	}
	return false
}

func addErrorEnvVar(spec *specs.Spec, msg string) {
	for _, env := range spec.Process.Env {
		if strings.HasPrefix(env, EnvHLRuntimeError) {
			return
		}
	}
	addEnvVar(spec, EnvHLRuntimeError, msg)
}
