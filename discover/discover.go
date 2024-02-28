package discover

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

var ErrNoDevices = errors.New("no habanalabs devices found. driver might not be loaded")

var SysClassAccel = "/sys/class/accel/accel"

// AcceleratorDevices finds the Habanalabs infiniband cards for the accelerators,
// and their control units, i.e accel0 and accel_controlD0
func AcceleratorDevices() []string {
	matches, err := filepath.Glob("/dev/accel/accel*")
	if err != nil {
		panic(err)
	}

	return matches
}

// InfinibandDevices finds the related Habanalabs infiniband cards for the accelerators.
func InfinibandDevices() []string {
	matches, err := filepath.Glob("/sys/class/infiniband/hlib_*")
	if err != nil {
		panic(err)
	}

	return matches
}

// UverbsForAccelerators returns list of infiniband char devices attached to each
// accelerator provided.
func UverbsForAccelerators(deviceIDs []string) ([]string, error) {
	var uverbDevices []string
	// For each devices, get its pci address
	for _, id := range deviceIDs {
		content, err := os.ReadFile(fmt.Sprintf("%s%s/device/pci_addr", SysClassAccel, id))
		if err != nil {
			return nil, err
		}
		pciAddr := strings.TrimSpace(string(content))

		// Check at the device folder for the uverb dir
		deviceUverbPath := fmt.Sprintf("/sys/bus/pci/devices/%s/infiniband_verbs", pciAddr)

		uverbs, err := os.ReadDir(deviceUverbPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if len(uverbs) == 0 {
			continue
		}
		uverbDevices = append(uverbDevices, fmt.Sprintf("/dev/infiniband/%s", uverbs[0].Name()))
	}
	return uverbDevices, nil
}

// Extract network interfaces names from hlib device.
func ExternalInterfaces(absHlibDevicePaths []string) ([]string, error) {
	var discoveredDevices []string
	for _, hlib := range absHlibDevicePaths {
		if !path.IsAbs(hlib) {
			return nil, fmt.Errorf("path provided is not absolute: %s", hlib)
		}
		netDevDir := fmt.Sprintf("%s/device/net", hlib)
		devices, err := os.ReadDir(netDevDir)
		if err != nil {
			return nil, fmt.Errorf("discovering external devices: %v", err)
		}
		for _, netDev := range devices {
			discoveredDevices = append(discoveredDevices, netDev.Name())
		}
	}
	return discoveredDevices, nil
}

func CharDevices(prefixes []string) ([]string, error) {
	var devices []string
	for _, prefix := range prefixes {
		matches, err := filepath.Glob(fmt.Sprintf("/dev/%s[0-9]", prefix))
		if err != nil {
			return nil, fmt.Errorf("discover accelerators: %w", err)
		}
		devices = append(devices, matches...)
	}
	if len(devices) == 0 {
		return nil, ErrNoDevices
	}
	return devices, nil
}

// DevicesIDs returns the unique device ids on the system
func DevicesIDs(devices []string) []string {
	var unique []string
	m := make(map[string]bool)

	for _, dev := range devices {
		devID := string(dev[len(dev)-1])
		if !m[devID] {
			m[devID] = true
			unique = append(unique, devID)
		}
	}
	return unique
}

// AcceleratorModuleID returns the module ID (OAM) of the requested accelerator.
func AcceleratorModuleID(acceleratorID string) (string, error) {
	modPath := fmt.Sprintf("%s%s/device/module_id", SysClassAccel, acceleratorID)
	content, err := os.ReadFile(path.Clean(modPath))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading modulde_id file: %v", err)
	}
	return strings.TrimSpace(string(content)), nil
}

type DevInfo struct {
	Path     string
	Major    uint32
	Minor    uint32
	Mode     uint32
	Uid      uint32
	Gid      uint32
	FileMode os.FileMode
}

func DeviceInfo(filepath string) (*DevInfo, error) {
	var stat unix.Stat_t
	err := unix.Lstat(filepath, &stat)
	if err != nil {
		return nil, fmt.Errorf("device info: %w", err)
	}

	devNum := stat.Rdev
	mode := stat.Mode
	switch mode & unix.S_IFMT {
	case unix.S_IFBLK:
		mode |= unix.S_IFBLK
	case unix.S_IFCHR:
		mode |= unix.S_IFCHR
	case unix.S_IFIFO:
		mode |= unix.S_IFIFO
	default:
		return nil, fmt.Errorf("not a device")
	}

	major := unix.Major(devNum)
	minor := unix.Minor(devNum)

	if major == 0xFFFFFFFF || minor == 0xFFFFFFFF {
		return nil, fmt.Errorf("devInfo: cannot mkdev. device in invalid state: %d,%d", major, minor)
	}

	return &DevInfo{
		Path:     filepath,
		Major:    major,
		Minor:    minor,
		Mode:     mode,
		Uid:      stat.Uid,
		Gid:      stat.Gid,
		FileMode: os.FileMode(stat.Mode),
	}, nil
}
