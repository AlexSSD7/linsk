package vm

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/AlexSSD7/linsk/qemucli"
)

func configureBaseVMCmd(logger *slog.Logger, cfg VMConfig) (string, []qemucli.Arg, error) {
	baseCmd := "qemu-system"

	if runtime.GOOS == "windows" {
		baseCmd += ".exe"
	}

	args := []qemucli.Arg{
		qemucli.MustNewStringArg("serial", "stdio"),
		qemucli.MustNewUintArg("m", cfg.MemoryAlloc),
		qemucli.MustNewUintArg("m", runtime.NumCPU()),
	}

	var accel string
	switch runtime.GOOS {
	case "windows":
		// TODO: To document: For Windows, we need to install QEMU using an installer and add it to PATH.
		// Then, we should enable Windows Hypervisor Platform in "Turn Windows features on or off".
		// IMPORTANT: We should also install libusbK drivers for USB devices we want to pass through.
		// This can be easily done with a program called Zadiag by Akeo.
		accel = "whpx,kernel-irqchip=off"
	case "darwin":
		accel = "hvf"
	default:
		accel = "kvm"
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
			qemucli.MustNewMapArg("machine", map[string]string{"type": "virt", "highmem": "off"}),
			qemucli.MustNewStringArg("cpu", "host"),
		)

		baseCmd += "-aarch64"
	default:
		return "", nil, fmt.Errorf("arch '%v' is not supported", runtime.GOARCH)
	}

	args = append(args, qemucli.MustNewStringArg("accel", accel))

	if cfg.BIOSPath != "" {
		args = append(args, qemucli.MustNewStringArg("bios", filepath.Clean(cfg.BIOSPath)))
	}

	return baseCmd, args, nil
}
