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
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math/rand"
	"net"
	"os/exec"
	"syscall"

	"golang.org/x/exp/slices"

	"github.com/HabanaAI/habana-container-runtime/discover"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func exposeInterfaces(logger *slog.Logger, pid int, requestedDevs []string) error {
	logger.Info("Exposing interfaces")

	netNS := fmt.Sprintf("/proc/%d/ns/net", pid)
	logger.Debug("Found network namespace", "path", netNS)

	// Filter network devices based on requested accelerators.
	hlibDevices := filterDevicesByENV(requestedDevs, discover.InfinibandDevices())
	logger.Info(
		"Exposing interfaces for devices",
		"requested_devices", requestedDevs,
		"hlib_devices", hlibDevices,
	)

	extInts, err := discover.ExternalInterfaces(hlibDevices)
	if err != nil {
		return err
	}

	if len(extInts) == 0 {
		logger.Warn("External network is not available")
		return nil
	}

	logger.Info("Found external interfaces", "intfs", extInts)

	netns, err := ns.GetNS(netNS)
	if err != nil {
		return fmt.Errorf("getting container network namespace: %w", err)
	}

	// For each habana interface, create a pair.
	// We are exposing the interfaces inside the container using macvlan
	// with passthru approach, to keep the mac addresses.
	for _, hostIntf := range extInts {
		hostLink, err := netlink.LinkByName(hostIntf)
		if err != nil {
			return fmt.Errorf("getting link by name: %w", err)
		}

		// If link is down, skip on the device
		attrs := hostLink.Attrs()
		if attrs.Flags&net.FlagUp == 0 {
			logger.Warn("Device is down. Skipping", "interface", hostIntf)
			continue
		}

		// Temporary name is required for creating the link first on the host
		// before moving it to the container namespace.
		tempName := randomString(8)
		if len(tempName) > syscall.IFNAMSIZ {
			tempName = tempName[:syscall.IFNAMSIZ]
		}

		linkAttrs := netlink.LinkAttrs{
			Name:        tempName,
			ParentIndex: hostLink.Attrs().Index,
			MTU:         hostLink.Attrs().MTU,
			Namespace:   netlink.NsFd(int(netns.Fd())),
		}

		containerLink := &netlink.Macvlan{
			LinkAttrs: linkAttrs,
			Mode:      netlink.MACVLAN_MODE_PASSTHRU,
		}

		// Create the temporary link.
		err = netlink.LinkAdd(containerLink)
		if err != nil {
			return fmt.Errorf("failed creating temporary link on host: %w", err)
		}
		// Make sure we remove the created interface from the host.
		defer func() {
			_ = netlink.LinkDel(containerLink)
		}()

		devAddrs, err := netlink.AddrList(hostLink, 0)
		if err != nil {
			return err
		}
		logger.Info("Found ip addresses for interface", "interface", hostIntf, "addrs", devAddrs)

		var gaudiPeer *netlink.Route
		hostLinkRoute, err := netlink.RouteList(hostLink, 0)
		if err != nil {
			return fmt.Errorf("failed getting route: %w", err)
		}

		for i, r := range hostLinkRoute {
			if r.Gw != nil {
				logger.Info("Found route address for ip", "interface", hostIntf, "route", r.String())
				gaudiPeer = &hostLinkRoute[i]
				break
			}
		}

		// Running commands inside the container namespaces.
		err = netns.Do(func(_ ns.NetNS) error {
			cl, err := netlink.LinkByName(tempName)
			if err != nil {
				return err
			}

			logger.Info("Setting link name inside namespace", "current_name", cl.Attrs().Name, "new_name", hostIntf)
			err = netlink.LinkSetName(cl, hostIntf)
			if err != nil {
				// If device with the exact same name exists inside the container, either it's
				// hostNetwork that is being used, or another CNI used our device name (In theory should
				// not happen).
				if errors.Is(err, fs.ErrExist) {
					logger.Info("device already exists in namespace. Host network used?")
					return nil
				}
				return err
			}

			// Get the link from inside the namespace to refresh all properties.
			cl, err = netlink.LinkByName(hostIntf)
			if err != nil {
				return err
			}

			// Add the same IP address of the host.
			if gaudiPeer != nil {
				logger.Info("Adding address to interface", "interface", hostIntf, "addr", devAddrs[0].IP.String())
				err = netlink.AddrAdd(cl, &devAddrs[0])
				if err != nil {
					return err
				}
			}

			err = netlink.LinkSetUp(cl)
			if err != nil {
				return err
			}

			if gaudiPeer != nil {
				// TODO: Use netlink package
				command := fmt.Sprintf("ip route append %s via %s dev %s", gaudiPeer.Dst.String(), gaudiPeer.Gw.String(), hostIntf)

				logger.Info("Adding route for device", "interface", hostIntf, "command", command)

				err = execute(command)
				if err != nil {
					return fmt.Errorf("appending route: %w", err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func execute(command string) error {
	args := append([]string{"/bin/bash", "-c"}, command)
	cmd := exec.Command(args[0], args[1:]...)
	content, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(string(content))
	}
	return nil
}

func filterDevicesByENV(requestedDevs, devices []string) []string {
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

var letters = []rune("abcdefghijflmnopqrstuvwxyz")

func randomString(n int) string {
	var s string
	for i := 0; i < n; i++ {
		letter := letters[rand.Intn(len(letters))]
		s += string(letter)
	}
	return s
}
