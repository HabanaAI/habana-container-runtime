package main

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/HabanaAI/habana-container-runtime/cgroup"
	"github.com/HabanaAI/habana-container-runtime/discover"
	"github.com/HabanaAI/habana-container-runtime/netinfo"

	"github.com/urfave/cli/v2"
)

const (
	// Prestart hook is deprecated in newer verions of the spec.
	HookPrestart = "prestart"
	// Create runtime hook is where we create the network devices.
	HookCreateRuntime = "createRuntime"
)

var (
	devPrefixes  = []string{"accel/accel", "accel/accel_controlD"}
	ErrNoDevices = errors.New("no habanalabs devices found. driver might not be loaded")
)

type config struct {
	// Requested hook cycle
	hook string
	// Device flag
	device string
	// Container PID
	pid int
	// Log file path
	logFilePath string
	// Gaudinet file path for l3.
	gaudinetFile string
	// Indicates whether or not running inside kubernetes environment where
	// Habana device plugin exists
	mountAccelerators bool
	// Mount Infiniband uverbs devices
	mountUverbs bool
}

func main() {
	var cfg config

	app := &cli.App{
		Name:      "habana-container-cli",
		Usage:     "Mount HabanaLabs devices into containers",
		UsageText: "habana-container-cli [global options] rootfs",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "hook",
				Usage:       "The runtime hook name. Support valued are \"prestart\" for legacy or \"createContainer\"",
				Destination: &cfg.hook,
				Value:       HookCreateRuntime,
				Action: func(_ *cli.Context, s string) error {
					if s != HookCreateRuntime && s != HookPrestart {
						return fmt.Errorf("unssuported hook type. valid types are %q and %q", HookCreateRuntime, HookPrestart)
					}
					return nil
				},
			},
			&cli.IntFlag{
				Name:        "pid",
				Usage:       "Container `PID`",
				Required:    true,
				Destination: &cfg.pid,
				Action: func(_ *cli.Context, i int) error {
					if i <= 0 {
						return fmt.Errorf("invalid pid. must be greater than 0")
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:        "device",
				Usage:       "Comma separated devices",
				Required:    true,
				Value:       "all",
				Destination: &cfg.device,
				Action: func(_ *cli.Context, s string) error {
					if s != "all" {
						for _, i := range strings.Split(s, ",") {
							if _, err := strconv.Atoi(i); err != nil {
								return fmt.Errorf("device value must be a number. invalid value: %s", i)
							}
						}
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:        "debug",
				Usage:       "Debug log file location",
				Value:       "/dev/null",
				Destination: &cfg.logFilePath,
			},
			&cli.StringFlag{
				Name:        "routes-files",
				Usage:       "Gaudinet file path",
				Value:       "",
				Destination: &cfg.gaudinetFile,
			},
			&cli.BoolFlag{
				Name:        "mount-accelerators",
				Usage:       "Enable or disable mounting Habanalabs Accelerator devices.",
				Value:       true,
				Destination: &cfg.mountAccelerators,
			},
			&cli.BoolFlag{
				Name:        "mount-uverbs",
				Usage:       "Mount accelerators' attached infiniband verb devices",
				Value:       true,
				Destination: &cfg.mountUverbs,
			},
		},
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() == 0 {
				return fmt.Errorf("missing rootfs argument")
			}

			logger, cleanup, err := initLogger(cfg.logFilePath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return err
			}
			defer cleanup()

			if err := run(ctx.Args().Get(0), cfg, logger); err != nil {
				logger.Error(err.Error())
				return err
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initLogger(filePath string) (*slog.Logger, func(), error) {
	// Open the log file for write in the specified location
	logFile, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to open debug log file: %v", err)
	}

	// Initialize the logger and the output for the log file
	log := slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{AddSource: false}))

	return log, func() { _ = logFile.Close() }, nil
}

func run(rootfs string, config config, logger *slog.Logger) error {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(fmt.Sprintf("panic: %s", err))
		}
	}()

	devices, err := parseDevices(logger, config.device)
	if err != nil {
		// Driver is not loaded, or the requested container is not habana related,
		// we'll print to the log and continue.
		logger.Error(err.Error())
		if errors.Is(err, discover.ErrNoDevices) {
			return nil
		}
		return err
	}
	logger.Info("Requested devices", "devices", devices)

	// If it's a 'prestart' hook, meaning user need to run the cli in
	// legacy mode, so all the devices mount happen here and not in the runtime.
	if config.hook == HookPrestart {
		err := handlePrestart(logger, rootfs, config, devices)
		if err != nil {
			return fmt.Errorf("handling prestart hook: %w", err)
		}
	}

	// In both types of hooks, we handle the exposure of the network interfaces
	// inside the container.
	err = exposeInterfaces(logger, config.pid, discover.DevicesIDs(devices.accelerators))
	if err != nil {
		return fmt.Errorf("exposing interfaces: %w", err)
	}

	return nil
}

type availableDevices struct {
	accelerators []string
	uverbs       []string
}

func handlePrestart(logger *slog.Logger, rootfs string, config config, devices availableDevices) error {
	// determine cgroup version
	cgroupVersion, err := cgroup.CGroupVersion("/", config.pid)
	if err != nil {
		return fmt.Errorf("handle prestart: %w", err)
	}
	logger.Info(fmt.Sprintf("Detected cgroup version: %d", cgroupVersion))

	handler, err := cgroup.New(cgroupVersion)
	if err != nil {
		return fmt.Errorf("handle prestart: %w", err)
	}

	cgrpMountPath, cgrpRootPrefix, err := handler.DeviceCGroupMountPath("/", config.pid)
	if err != nil {
		return fmt.Errorf("handle prestart: %w", err)
	}

	cgrpRootPath, err := handler.DeviceCGroupRootPath("/", cgrpRootPrefix, config.pid)
	if err != nil {
		return fmt.Errorf("handle prestart: %w", err)
	}

	containerCgroupPath := path.Join(cgrpRootPrefix, cgrpRootPath)

	logger.Info(
		"Prestart request details",
		"rootfs", rootfs,
		"cgroup_root_prefix", cgrpRootPrefix,
		"cgroup_mount_path", cgrpMountPath,
		"container_cgroup_path", containerCgroupPath,
	)

	// If we are not running inside kubernetes environment, then we should mount
	// the devices into the container. Otherwise, this is done by device plugin.
	if config.mountAccelerators {
		if err := handleMounts(logger, handler, devices.accelerators, rootfs, config.pid, containerCgroupPath); err != nil {
			return fmt.Errorf("handle prestart: %w", err)
		}
	}

	if config.mountUverbs {
		if err := handleMounts(logger, handler, devices.uverbs, rootfs, config.pid, containerCgroupPath); err != nil {
			return fmt.Errorf("handle prestart: %w", err)
		}
	}

	// net info
	err = netinfo.Generate(discover.DevicesIDs(devices.accelerators), rootfs)
	if err != nil {
		logger.Error(fmt.Sprintf("ERROR adding netinfo: %v", err))
	} else {
		logger.Info("Added network information")
	}

	if config.gaudinetFile != "" {
		err = netinfo.GaudinetFile(logger, rootfs, config.gaudinetFile)
		if err != nil {
			logger.Error(fmt.Sprintf("copying gaudinet file: %v", err))
		}
	}

	logger.Info("Completed prestart hook")
	return nil
}

// parseDevices returns the devices number selected by the user.
func parseDevices(logger *slog.Logger, deviceFlag string) (availableDevices, error) {
	// TODO: separate prefixes and collect each on its own. Check
	// length of both matches, if not, return error about driver.
	accelDevices, err := discover.CharDevices(devPrefixes)
	if err != nil {
		return availableDevices{}, err
	}
	logger.Info("Available accelerators devices on machines", "accelerators", accelDevices)

	var devices []string
	if deviceFlag == "all" {
		devices = append(devices, accelDevices...)
	} else {
		// Filter requested devices
		for _, i := range strings.Split(deviceFlag, ",") {
			for _, dev := range accelDevices {
				if i == deviceID(dev) {
					devices = append(devices, dev)
				}
			}
		}
	}

	deviceIds := discover.DevicesIDs(devices)
	logger.Info("Device IDs after filter", "ids", deviceIds)

	// Always add the MOFED devices is exists
	verbs, err := discover.UverbsForAccelerators(deviceIds)
	if err != nil {
		return availableDevices{}, err
	}

	return availableDevices{
		accelerators: devices,
		uverbs:       verbs,
	}, nil
}

func handleMounts(logger *slog.Logger, handler cgroup.Handler, devices []string, rootfs string, pid int, cgroupPath string) error {
	devicesInfo, err := mountDevices(logger, devices, rootfs, pid)
	if err != nil {
		return err
	}
	logger.Info("mounted all devices")

	// Convert device info into cgroup device rules
	rules := convertDevInfoToDevRule(devicesInfo)

	// Add rules to container cgroup
	err = handler.AddDeviceRules(cgroupPath, rules)
	if err != nil {
		return err
	}
	logger.Info("created devices rules in cgroup")
	return nil
}

// mountHLDevice handles the creation of all requested Habana devices inside the container.
func mountDevices(logger *slog.Logger, devices []string, rootfs string, pid int) ([]*discover.DevInfo, error) {
	var infos []*discover.DevInfo
	for _, dev := range devices {
		info, err := mountHLDevice(logger, rootfs, pid, dev)
		if err != nil {
			return nil, fmt.Errorf("mountDevices: %w", err)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// mountHLDevice creates the device inside the container root file system.
func mountHLDevice(logger *slog.Logger, rootfs string, pid int, hostPath string) (*discover.DevInfo, error) {
	if !fileExist(hostPath) {
		return nil, fmt.Errorf("mountHLDevice: requested device does not exist: %v", hostPath)
	}

	hostPath = path.Clean(hostPath)
	containerPath := path.Join(rootfs, hostPath)
	logger.Info("Trying to mount accel device", "host_path", hostPath, "container_path", containerPath)

	// Get the device information
	info, err := discover.DeviceInfo(hostPath)
	if err != nil {
		return nil, err
	}

	// Create and mount the device inside the container, if it's not already mounted
	if !fileExist(containerPath) {
		if err := mountInContainer(pid, hostPath, containerPath); err != nil {
			return nil, err
		}
		logger.Info("Created container path", "path", containerPath)
	}
	return info, nil
}

func fileExist(filepath string) bool {
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return false
	}
	return true
}

// mountInContainer mounted the requested devices inside the container namespace matching the PID
func mountInContainer(pid int, hostPath, containerPath string) error {
	var errBuf bytes.Buffer

	// We need to make sure the device directory exists in the container namespace.
	// In case for hlX it's /dev and in accelX it's /dev/accel/
	if !fileExist(path.Dir(containerPath)) {
		mkdir := exec.Command("nsenter", "-m", "-t", fmt.Sprintf("%d", pid),
			"mkdir", "-p", path.Dir(containerPath))
		mkdir.Stderr = &errBuf
		if err := mkdir.Run(); err != nil {
			return fmt.Errorf("error running the mkdir command %v: %v", mkdir.Args, errBuf.String())
		}
		errBuf.Reset()
	}

	// Create the device file in the container namespace
	createFile := exec.Command("nsenter", "-m", "-t", fmt.Sprintf("%d", pid),
		"touch", containerPath)
	createFile.Stderr = &errBuf
	if err := createFile.Run(); err != nil {
		return fmt.Errorf("error running the createFile command %v: %v", createFile.Args, errBuf.String())
	}
	errBuf.Reset()

	// Mount the host's device into the container namespace
	mountDevice := exec.Command("nsenter", "-m", "-t", fmt.Sprintf("%d", pid),
		"mount", "--bind", hostPath, containerPath)
	mountDevice.Stderr = &errBuf
	removeFile := exec.Command("nsenter", "-m", "-t", fmt.Sprintf("%d", pid),
		"rm", containerPath)

	if err := mountDevice.Run(); err != nil {
		_ = removeFile.Run()
		return fmt.Errorf("error running the mountDevice command %v: %v", mountDevice.Args, errBuf.String())
	}

	return nil
}

// convertDevInfoToDevRule converts the collected devices information to linux security
// groups to allow access through cgroup.
func convertDevInfoToDevRule(devices []*discover.DevInfo) []cgroup.DeviceRule {
	rules := make([]cgroup.DeviceRule, len(devices))
	for i := 0; i < len(devices); i++ {
		rule := cgroup.DeviceRule{
			Allow:  true,
			Type:   "c",
			Access: "rwm",
			Major:  func() *int64 { x := int64(devices[i].Major); return &x }(),
			Minor:  func() *int64 { x := int64(devices[i].Minor); return &x }(),
		}
		rules[i] = rule
	}
	return rules
}

// deviceID extracts the numeric value of the accelerator devices.
// /dev/accel0 --> 0, /dev/accel_controlD0 --> 0
func deviceID(device string) string {
	// Clean root dev folder
	cleanDev := strings.TrimPrefix(device, "/dev/")
	// Clean accel folder and device
	cleanDev = strings.TrimPrefix(cleanDev, "accel/accel")
	// Clean control prefix for controlD devices
	cleanDev = strings.TrimPrefix(cleanDev, "_controlD")
	return cleanDev
}
