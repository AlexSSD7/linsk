package cmd

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/AlexSSD7/linsk/share"
	"github.com/AlexSSD7/linsk/vm"
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

			fpr, err := vm.ParsePortForwardString(fp)
			if err != nil {
				slog.Error("Failed to parse port forward string", "index", i, "value", fp, "error", err.Error())
				os.Exit(1)
			}

			forwardPortRules = append(forwardPortRules, fpr)
		}

		os.Exit(runVM(passthroughArg, func(ctx context.Context, i *vm.VM, fm *vm.FileManager, trc *share.NetTapRuntimeContext) int {
			sc, err := i.DialSSH()
			if err != nil {
				slog.Error("Failed to dial VM SSH", "error", err.Error())
				return 1
			}

			if trc != nil {
				slog.Info("Tap networking is active", "host-ip", trc.Net.HostIP, "vm-ip", trc.Net.GuestIP)
			}

			defer func() { _ = sc.Close() }()

			sess, err := sc.NewSession()
			if err != nil {
				slog.Error("Failed to create new VM SSH session", "error", err.Error())
				return 1
			}

			defer func() { _ = sess.Close() }()

			termFD := int(os.Stdin.Fd())
			termState, err := term.MakeRaw(termFD)
			if err != nil {
				slog.Error("Failed to make raw terminal", "error", err.Error())
				return 1
			}

			defer func() {
				err := term.Restore(termFD, termState)
				if err != nil {
					slog.Error("Failed to restore terminal", "error", err.Error())
				}
			}()

			termFDGetSize := termFD
			if runtime.GOOS == "windows" {
				// Another Windows workaround :/
				termFDGetSize = int(os.Stdout.Fd())
			}

			termWidth, termHeight, err := term.GetSize(termFDGetSize)
			if err != nil {
				slog.Error("Failed to get terminal size", "error", err.Error())
				return 1
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
				slog.Error("Failed to request VM SSH pty", "error", err.Error())
				return 1
			}

			sess.Stdin = os.Stdin
			sess.Stdout = os.Stdout
			sess.Stderr = os.Stderr

			err = sess.Shell()
			if err != nil {
				slog.Error("Start VM SSH shell", "error", err.Error())
				return 1
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
