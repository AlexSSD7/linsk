package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/AlexSSD7/linsk/vm"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var shellCmd = &cobra.Command{
	Use: "shell",
	// TODO: Fill this
	// Short: "",
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var passthroughArg string
		if len(args) > 0 {
			passthroughArg = args[0]
		}

		os.Exit(runVM(passthroughArg, func(ctx context.Context, i *vm.Instance, fm *vm.FileManager) int {
			sc, err := i.DialSSH()
			if err != nil {
				slog.Error("Failed to dial VM SSH", "error", err)
				return 1
			}

			defer func() { _ = sc.Close() }()

			sess, err := sc.NewSession()
			if err != nil {
				slog.Error("Failed to create new VM SSH session", "error", err)
				return 1
			}

			defer func() { _ = sess.Close() }()

			termFD := int(os.Stdin.Fd())
			termState, err := term.MakeRaw(termFD)
			if err != nil {
				slog.Error("Failed to make raw terminal", "error", err)
				return 1
			}

			defer func() {
				err := term.Restore(termFD, termState)
				if err != nil {
					slog.Error("Failed to restore terminal", "error", err)
				}
			}()

			termWidth, termHeight, err := term.GetSize(termFD)
			if err != nil {
				slog.Error("Failed to get terminal size", "error", err)
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
				slog.Error("Failed to request VM SSH pty", "error", err)
				return 1
			}

			sess.Stdin = os.Stdin
			sess.Stdout = os.Stdout
			sess.Stderr = os.Stderr

			err = sess.Shell()
			if err != nil {
				slog.Error("Start VM SSH shell", "error", err)
				return 1
			}

			doneCh := make(chan struct{}, 1)

			go func() {
				err = sess.Wait()
				if err != nil {
					slog.Error("Failed to wait for VM SSH session to finish", "error", err)
				}

				doneCh <- struct{}{}
			}()

			select {
			case <-ctx.Done():
			case <-doneCh:
			}

			return 0
		}, nil))

		return nil
	},
}

var forwardPortsFlagStr string

func init() {
	shellCmd.Flags().StringVar(&forwardPortsFlagStr, "forward-ports", "", "Extra TCP port forwarding rules. Syntax: '<HOST PORT>:<VM PORT>' OR '<HOST BIND IP>:<HOST PORT>:<VM PORT>'. Multiple rules split by comma are accepted.")
}
