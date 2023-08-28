// go:build windows

package vm

import (
	"fmt"
	"os/exec"
	"syscall"
)

func prepareVMCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func terminateProcess(pid int) error {
	return exec.Command("TASKKILL", "/T", "/F", "/PID", fmt.Sprint(pid)).Run()
}
