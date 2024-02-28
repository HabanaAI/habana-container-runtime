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
package netinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
)

// osReadFile is overwritten in tests.
var osReadFile = os.ReadFile

// hlsNumInterfaceByType hold the known number of network ports (internal+external)
// for each Gaudi device we have
var hlsNumInterfaceByType = map[string]int{
	"gaudi":  10,
	"gaudi2": 24,
}

type MACInfo struct {
	PCI_ID        string
	MAC_ADDR_LIST []string
}

type NetJSON struct {
	MAC_ADDR_INFO []MACInfo
}

// Generates creates the mac address information for the requested accelerator devices.
func Generate(devices []string, containerRootFS string) error {
	basePath := path.Join(containerRootFS, "/etc/habanalabs/")
	netFilePath := path.Join(basePath, "macAddrInfo.json")

	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		if err := os.Mkdir(basePath, 0750); err != nil {
			return err
		}
	}

	netData, err := netConfig(devices)
	if err != nil {
		return err
	}

	// Write the file only if we have data about the MAC when the driver is loaded
	if strings.Contains(netData, "MAC_ADDR_LIST") {
		netConfigFile, err := os.OpenFile(path.Clean(netFilePath), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("fail to open netConfig json file: %w", err)
		}
		defer netConfigFile.Close()

		_, err = netConfigFile.WriteString(netData)
		if err != nil {
			return fmt.Errorf("failed writing macAddrInfo.json: %w", err)
		}
	}

	return nil
}

func GaudinetFile(logger *slog.Logger, containerRootFS, source string) error {
	// Destination inside the container file system.
	destFile := path.Join(containerRootFS, "etc", "habanalabs", "gaudinet.json")

	info, err := os.Stat(source)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logger.Info(fmt.Sprintf("file does not exist on host: %s", source))
			return nil
		}
		return err
	}

	// Skip copying an empty file to avoid HCL problem
	if info.Size() == 0 {
		logger.Info("File exists but it's empty, skiping...")
		return nil
	}

	srcFile, err := os.Open(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer srcFile.Close()

	dst, err := os.OpenFile(path.Clean(destFile), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, srcFile)
	if err != nil {
		return err
	}

	return nil
}

func netConfig(devices []string) (string, error) {
	if len(devices) == 0 {
		return "", nil
	}

	deviceType, err := deviceType(devices[0])
	if err != nil {
		return "", fmt.Errorf("netConfig: %w", err)
	}

	devicesPCI, err := devicesPCIAddresses(devices)
	if err != nil {
		return "", err
	}

	netInfo, err := devicesMACAddress(devicesPCI, deviceType)
	if err != nil {
		return "", err
	}

	jsondat := &NetJSON{MAC_ADDR_INFO: netInfo}
	encjson, _ := json.MarshalIndent(jsondat, "", "    ")
	return string(encjson), nil
}

func deviceType(deviceID string) (string, error) {
	content, err := osReadFile(fmt.Sprintf("/sys/class/accel/accel%s/device/device_type", deviceID))
	if err != nil {
		return "", fmt.Errorf("deviceType: %w", err)
	}

	data := strings.TrimSpace(string(content))

	parts := strings.Fields(data)
	if len(parts) == 0 {
		return "", fmt.Errorf("deviceType info not found")
	}

	return strings.ToLower(parts[0]), nil
}

// returns a map of requested Habana devices in the form of map[hlID]pciAddress
func devicesPCIAddresses(devices []string) (map[string]string, error) {
	pciInfo := make(map[string]string)

	for _, devID := range devices {
		devName := "accel" + devID
		content, err := osReadFile(path.Clean(path.Join("/sys/class/accel", devName, "device", "pci_addr")))
		if err != nil {
			return nil, err
		}
		pciAddr := strings.TrimSpace(string(content))
		pciInfo[devID] = pciAddr
	}

	return pciInfo, nil
}

func devicesMACAddress(pciDevices map[string]string, devType string) ([]MACInfo, error) {
	var devInfo []MACInfo

	// Collect external ports mac addresses
	extPorts, err := extPortsMACAddress(pciDevices)
	if err != nil {
		return devInfo, err
	}

	// Fill MAC addresses data based on port type external or internal
	for hlID, pciID := range pciDevices {
		var macAddressList []string

		for i := 0; i < hlsNumInterfaceByType[devType]; i++ {
			// If the port is recognized as external, we add the readl mac addresss,
			// otherwise, we add a broadcast mac address for each internal port
			if _, exists := extPorts[hlID][i]; exists {
				macAddressList = append(macAddressList, extPorts[hlID][i])
			} else {
				macAddressList = append(macAddressList, "ff:ff:ff:ff:ff:ff")
			}
		}

		devInfo = append(devInfo, MACInfo{
			PCI_ID:        pciID,
			MAC_ADDR_LIST: macAddressList,
		})
	}

	return devInfo, nil
}

// getExtPorts receives Habana list of devices, and returns their MacAddress and device port
// of their external interfaces. Returns map[hlID]map[devPort]PciAddr
func extPortsMACAddress(pciDevices map[string]string) (map[string]map[int]string, error) {
	extInfo := make(map[string]map[int]string)

	for hlID, pci := range pciDevices {
		netPath := fmt.Sprintf("%s/%s/net", "/sys/bus/pci/devices", pci)

		ifaces, err := os.ReadDir(netPath)
		if err != nil {
			return nil, err
		}

		netinfo := make(map[int]string)
		for _, inet := range ifaces {
			// Get MAC Address
			mac, err := os.ReadFile(path.Clean(path.Join(netPath, inet.Name(), "address")))
			if err != nil {
				return nil, err
			}

			// Get dev port
			devPort, err := os.ReadFile(path.Clean(path.Join(netPath, inet.Name(), "dev_port")))
			if err != nil {
				return nil, err
			}
			devPortInt, err := strconv.Atoi(strings.TrimSpace(string(devPort)))
			if err != nil {
				return nil, err
			}
			macAddr := strings.TrimSpace(string(mac))
			netinfo[devPortInt] = macAddr
		}
		extInfo[hlID] = netinfo

	}
	return extInfo, nil
}
