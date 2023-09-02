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

package cmd

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/AlexSSD7/linsk/osspecifics"
	"github.com/AlexSSD7/linsk/share"
	"github.com/AlexSSD7/linsk/vm"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Start a VM and access the shell. Useful for formatting drives and debugging.",
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		var passthroughArg string
		if len(args) > 0 {
			passthroughArg = args[0]
		}

		var forwardPortRules []vm.PortForwardingRule

		for i, fp := range strings.Split(forwardPortsFlagStr, ",") {
			if fp == "" {
				continue
			}

			fpr, err := vm.ParsePortForwardingRuleString(fp)
			if err != nil {
				slog.Error("Failed to parse port forwarding rule string", "index", i, "value", fp, "error", err.Error())
				os.Exit(1)
			}

			forwardPortRules = append(forwardPortRules, fpr)
		}

		os.Exit(runVM(passthroughArg, func(ctx context.Context, i *vm.VM, fm *vm.FileManager, trc *share.NetTapRuntimeContext) int {
			if trc != nil {
				slog.Info("Tap host-VM networking is active", "host-ip", trc.Net.HostIP, "vm-ip", trc.Net.GuestIP)
			}

			err := runVMShell(ctx, i)
			if err != nil {
				slog.Error("Failed to run VM shell", "error", err.Error())
				return 1
			}

			return 0
		}, forwardPortRules, true, enableTapNetFlag))
	},
}

var forwardPortsFlagStr string
var enableTapNetFlag bool

func init() {
	shellCmd.Flags().StringVar(&forwardPortsFlagStr, "forward-ports", "", "Extra TCP port forwarding rules. Syntax: '<HOST PORT>:<VM PORT>' OR '<HOST BIND IP>:<HOST PORT>:<VM PORT>'. Multiple rules split by comma are accepted.")
	shellCmd.Flags().BoolVar(&enableTapNetFlag, "enable-net-tap", false, "Enables host-VM tap networking.")
}

func runVMShell(ctx context.Context, vi *vm.VM) error {
	sc, err := vi.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial ssh")
	}

	defer func() { _ = sc.Close() }()

	sess, err := sc.NewSession()
	if err != nil {
		return errors.Wrap(err, "new vm ssh session")
	}

	defer func() { _ = sess.Close() }()

	termFD := int(os.Stdin.Fd())
	termState, err := term.MakeRaw(termFD)
	if err != nil {
		return errors.Wrap(err, "make raw terminal")
	}

	defer func() {
		err := term.Restore(termFD, termState)
		if err != nil {
			slog.Error("Failed to restore terminal", "error", err.Error())
		}
	}()

	termFDGetSize := termFD
	if osspecifics.IsWindows() {
		// Workaround for Windows.
		termFDGetSize = int(os.Stdout.Fd())
	}

	termWidth, termHeight, err := term.GetSize(termFDGetSize)
	if err != nil {
		return errors.Wrap(err, "get terminal size")
	}

	termModes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	term := os.Getenv("TERM")
	if term == "" {
		term = "xterm-256color"
	}

	err = sess.RequestPty(term, termHeight, termWidth, termModes)
	if err != nil {
		return errors.Wrap(err, "request vm ssh pty")
	}

	sess.Stdin = os.Stdin
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr

	err = sess.Shell()
	if err != nil {
		return errors.Wrap(err, "start vm ssh shell")
	}

	doneCh := make(chan struct{}, 1)

	go func() {
		err = sess.Wait()
		if err != nil {
			slog.Error("Failed to wait for VM SSH session to finish", "error", err.Error())
		}

		doneCh <- struct{}{}
	}()

	select {
	case <-ctx.Done():
	case <-doneCh:
	}

	return nil
}
