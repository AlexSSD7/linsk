// Linsk - A utility to access Linux-native file systems on non-Linux operating systems.
// Copyright (c) 2023 The Linsk Authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package vm

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/AlexSSD7/linsk/nettap"
	"github.com/AlexSSD7/linsk/osspecifics"
	"github.com/AlexSSD7/linsk/qemucli"
	"github.com/AlexSSD7/linsk/utils"
	"github.com/pkg/errors"
)

func getUniqueQEMUNetID() string {
	time.Sleep(time.Millisecond)
	return "net" + utils.IntToStr(time.Now().UnixNano())
}

func getUniqueQEMUDriveID() string {
	time.Sleep(time.Millisecond)
	return "drive" + utils.IntToStr(time.Now().UnixNano())
}

func cleanQEMUPath(s string) string {
	path := filepath.Clean(s)
	if osspecifics.IsWindows() {
		// QEMU doesn't work well with Windows backslashes, so we're replacing them to forward slashes
		// that work perfectly fine.
		path = strings.ReplaceAll(s, "\\", "/")
	}

	return path
}

func configureBaseVMCmd(logger *slog.Logger, cfg Config) (string, []qemucli.Arg, error) {
	baseCmd := "qemu-system"

	args := []qemucli.Arg{
		qemucli.MustNewStringArg("serial", "stdio"),
		qemucli.MustNewUintArg("m", cfg.MemoryAlloc),
		qemucli.MustNewUintArg("smp", runtime.NumCPU()),
	}

	if osspecifics.IsMacOS() {
		args = append(args, qemucli.MustNewStringArg("cpu", "host"))
	}

	var accel []qemucli.KeyValueArgItem
	switch {
	case osspecifics.IsWindows():
		accel = []qemucli.KeyValueArgItem{
			{Key: "whpx"},
			{Key: "kernel-irqchip", Value: "off"},
		}
	case osspecifics.IsMacOS():
		accel = []qemucli.KeyValueArgItem{{
			Key: "hvf",
		}}
	default:
		accel = []qemucli.KeyValueArgItem{{
			Key: "kvm",
		}}
	}

	switch runtime.GOARCH {
	case "amd64":
		baseCmd += "-x86_64"
	case "arm64":
		if cfg.BIOSPath == "" {
			logger.Warn("BIOS image path is not specified while attempting to run an aarch64 (arm64) VM. The VM will not boot.")
		}

		// "highmem=off" is required for M1.
		args = append(args,
			qemucli.MustNewKeyValueArg("machine", []qemucli.KeyValueArgItem{
				{Key: "type", Value: "virt"},
				{Key: "highmem", Value: "off"},
			}),
		)

		baseCmd += "-aarch64"
	default:
		return "", nil, fmt.Errorf("arch '%v' is not supported", runtime.GOARCH)
	}

	args = append(args, qemucli.MustNewKeyValueArg("accel", accel))

	if cfg.BIOSPath != "" {
		biosPath := cleanQEMUPath(cfg.BIOSPath)
		biosArg, err := qemucli.NewStringArg("bios", biosPath)
		if err != nil {
			return "", nil, errors.Wrapf(err, "create bios arg (path '%v')", biosPath)
		}

		args = append(args, biosArg)
	}

	if !cfg.Debug {
		args = append(args, qemucli.MustNewStringArg("display", "none"))
	}

	// TODO: There is no video configured by default on arm64, rendering --vm-debug useless.

	if cfg.CdromImagePath != "" {
		cdromPath := cleanQEMUPath(cfg.CdromImagePath)
		cdromArg, err := qemucli.NewStringArg("cdrom", cdromPath)
		if err != nil {
			return "", nil, errors.Wrapf(err, "create cdrom arg (path '%v')", cdromPath)
		}

		args = append(args, cdromArg, qemucli.MustNewStringArg("boot", "d"))
	}

	if osspecifics.IsWindows() {
		baseCmd += ".exe"
	}

	return baseCmd, args, nil
}

func configureVMCmdUserNetwork(ports []PortForwardingRule, unrestricted bool) ([]qemucli.Arg, error) {
	netID := getUniqueQEMUNetID()

	userNetdevValues := []qemucli.KeyValueArgItem{
		{Key: "type", Value: "user"},
		{Key: "id", Value: netID},
	}

	if !unrestricted {
		userNetdevValues = append(userNetdevValues, qemucli.KeyValueArgItem{Key: "restrict", Value: "on"})
	}

	for _, pf := range ports {
		hostIPStr := ""
		if pf.HostIP != nil {
			hostIPStr = pf.HostIP.String()
		}

		userNetdevValues = append(userNetdevValues, qemucli.KeyValueArgItem{
			Key:   "hostfwd",
			Value: "tcp:" + hostIPStr + ":" + utils.UintToStr(pf.HostPort) + "-:" + utils.UintToStr(pf.VMPort),
		})
	}

	netdevArg, err := qemucli.NewKeyValueArg("netdev", userNetdevValues)
	if err != nil {
		return nil, errors.Wrap(err, "create netdev key-value arg")
	}

	deviceArg, err := qemucli.NewKeyValueArg("device", []qemucli.KeyValueArgItem{{Key: "driver", Value: "virtio-net"}, {Key: "netdev", Value: netID}})
	if err != nil {
		return nil, errors.Wrap(err, "create device key-value arg")
	}

	args := []qemucli.Arg{
		netdevArg,
		deviceArg,
	}

	return args, nil
}

func configureVMCmdTapNetwork(tapName string) ([]qemucli.Arg, error) {
	err := nettap.ValidateTapName(tapName)
	if err != nil {
		return nil, errors.Wrapf(err, "validate network tap name '%v'", tapName)
	}

	netID := getUniqueQEMUNetID()

	netdevArg, err := qemucli.NewKeyValueArg("netdev", []qemucli.KeyValueArgItem{{Key: "type", Value: "tap"}, {Key: "id", Value: netID}, {Key: "ifname", Value: tapName}, {Key: "script", Value: "no"}, {Key: "downscript", Value: "no"}})
	if err != nil {
		return nil, errors.Wrap(err, "create netdev key-value arg")
	}

	deviceArg, err := qemucli.NewKeyValueArg("device", []qemucli.KeyValueArgItem{{Key: "driver", Value: "virtio-net"}, {Key: "netdev", Value: netID}})
	if err != nil {
		return nil, errors.Wrap(err, "create device key-value arg")
	}

	return []qemucli.Arg{netdevArg, deviceArg}, nil
}

func configureVMCmdNetworking(logger *slog.Logger, cfg Config, sshPort uint16) ([]qemucli.Arg, error) {
	// SSH port config.
	ports := []PortForwardingRule{{
		HostIP:   net.ParseIP("127.0.0.1"),
		HostPort: sshPort,
		VMPort:   22,
	}}

	ports = append(ports, cfg.ExtraPortForwardingRules...)

	if cfg.UnrestrictedNetworking {
		logger.Warn("Using unrestricted VM networking")
	}

	args, err := configureVMCmdUserNetwork(ports, cfg.UnrestrictedNetworking)
	if err != nil {
		return nil, errors.Wrap(err, "configure vm cmd user network")
	}

	for i, tap := range cfg.Taps {
		tapNetArgs, err := configureVMCmdTapNetwork(tap.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "configure tap network #%v", i)
		}

		args = append(args, tapNetArgs...)
	}

	return args, nil
}

func configureVMCmdDrives(cfg Config) ([]qemucli.Arg, error) {
	var args []qemucli.Arg

	for i, drive := range cfg.Drives {
		_, err := os.Stat(filepath.Clean(drive.Path))
		if err != nil {
			return nil, errors.Wrapf(err, "stat drive #%v path", i)
		}

		driveID := getUniqueQEMUDriveID()
		drivePath := cleanQEMUPath(drive.Path)

		driveKVItems := []qemucli.KeyValueArgItem{
			{Key: "file", Value: drivePath},
			{Key: "format", Value: "qcow2"},
			{Key: "if", Value: "none"},
			{Key: "id", Value: driveID},
		}

		if drive.SnapshotMode {
			driveKVItems = append(driveKVItems, qemucli.KeyValueArgItem{
				Key:   "snapshot",
				Value: "on",
			})
		}

		deviceKVItems := []qemucli.KeyValueArgItem{
			{Key: "driver", Value: "virtio-blk-pci"},
			{Key: "drive", Value: driveID},
		}

		if cfg.CdromImagePath == "" {
			deviceKVItems = append(deviceKVItems, qemucli.KeyValueArgItem{
				Key:   "bootindex",
				Value: utils.IntToStr(i),
			})
		}

		driveArg, err := qemucli.NewKeyValueArg("drive", driveKVItems)
		if err != nil {
			return nil, errors.Wrapf(err, "create drive key-value arg (path '%v')", drivePath)
		}

		deviceArg, err := qemucli.NewKeyValueArg("device", deviceKVItems)
		if err != nil {
			return nil, errors.Wrapf(err, "create device key-value arg (path '%v')", drivePath)
		}

		args = append(args, driveArg, deviceArg)
	}

	return args, nil
}

func configureVMCmdUSBPassthrough(cfg Config) []qemucli.Arg {
	var args []qemucli.Arg

	if len(cfg.PassthroughConfig.USB) != 0 {
		args = append(args, qemucli.MustNewKeyValueArg("device", []qemucli.KeyValueArgItem{{Key: "driver", Value: "nec-usb-xhci"}}))

		for _, dev := range cfg.PassthroughConfig.USB {
			args = append(args, qemucli.MustNewKeyValueArg("device", []qemucli.KeyValueArgItem{
				{Key: "driver", Value: "usb-host"},
				{Key: "vendorid", Value: "0x" + hex.EncodeToString(utils.Uint16ToBytesBE(dev.VendorID))},
				{Key: "productid", Value: "0x" + hex.EncodeToString(utils.Uint16ToBytesBE(dev.ProductID))},
			}))
		}
	}

	return args
}

func configureVMCmdBlockDevicePassthrough(logger *slog.Logger, cfg Config) ([]qemucli.Arg, error) {
	var args []qemucli.Arg

	if len(cfg.PassthroughConfig.Block) != 0 {
		logger.Warn("Using raw block device passthrough. Please note that it's YOUR responsibility to ensure that no device is mounted in your OS and the VM at the same time. Otherwise, you run serious risks. No further warnings will be issued.")
	}

	for _, dev := range cfg.PassthroughConfig.Block {
		// It's always a user's responsibility to ensure that no drives are mounted
		// in both host and guest system. This should serve as the last resort.
		{
			seemsMounted, err := osspecifics.CheckDeviceSeemsMounted(dev.Path)
			if err != nil {
				return nil, errors.Wrapf(err, "check whether device seems to be mounted (path '%v')", dev.Path)
			}

			if seemsMounted {
				return nil, fmt.Errorf("device '%v' seems to be already mounted in the host system", dev.Path)
			}
		}

		if dev.BlockSize == 0 {
			return nil, fmt.Errorf("invalid zero block size specified for device '%v'", dev.Path)
		}

		if dev.BlockSize > 65536 {
			return nil, fmt.Errorf("block size specified for device '%v' is too large (max is 65536): '%v'", dev.Path, dev.BlockSize)
		}

		if dev.BlockSize/512*512 != dev.BlockSize {
			return nil, fmt.Errorf("unaligned block size specified for device '%v' (must be in increments of 512): '%v'", dev.Path, dev.BlockSize)
		}

		strBlockSize := strconv.FormatUint(dev.BlockSize, 10)

		devPath := cleanQEMUPath(dev.Path)
		driveID := getUniqueQEMUDriveID()

		driveDevArg, err := qemucli.NewKeyValueArg("device", []qemucli.KeyValueArgItem{
			{Key: "driver", Value: "virtio-blk-pci"},
			{Key: "drive", Value: driveID},
			{Key: "logical_block_size", Value: strBlockSize},
			{Key: "physical_block_size", Value: strBlockSize},
		})
		if err != nil {
			return nil, errors.Wrapf(err, "create drive device key-value arg (path '%v')", devPath)
		}

		driveArg, err := qemucli.NewKeyValueArg("drive", []qemucli.KeyValueArgItem{
			{Key: "file", Value: devPath},
			{Key: "format", Value: "raw"},
			{Key: "if", Value: "none"},
			{Key: "id", Value: driveID},
		})
		if err != nil {
			return nil, errors.Wrapf(err, "create drive key-value arg (path '%v')", devPath)
		}

		args = append(args, driveDevArg, driveArg)
	}

	return args, nil
}
