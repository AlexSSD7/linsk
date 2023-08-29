package cmd

import (
	"os"

	"log/slog"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "linsk",
	Short: "Access Linux-native file system infrastructure (including LVM and LUKS) on macOS and Linux. Powered by a lightweight Alpine Linux VM and FTP.",
	Long: `Linsk is a utility that allows you to access Linux-native file system infrastructure, including device mapping technologies like LVM and LUKS without compromise on other operating systems that have little ` +
		`to no support for Linux's wide range of file systems, mainly aiming macOS and Windows. Linsk does not reimplement any file system. Instead, Linsk ` +
		`utilizes a lightweight Alpine Linux VM to tap into the native Linux software ecosystem. The files are then exposed to the host via fast and widely-supported FTP, ` +
		`operating at near-hardware speeds.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var vmDebugFlag bool
var unrestrictedNetworkingFlag bool
var vmMemAllocFlag uint64

// TODO: Version command.

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(shellCmd)

	rootCmd.PersistentFlags().BoolVar(&vmDebugFlag, "vmdebug", false, "Enables the VM debug mode. This will open an accessible VM monitor. You can log in with root user and no password.")
	rootCmd.PersistentFlags().BoolVar(&unrestrictedNetworkingFlag, "unrestricted-networking", false, "Enables unrestricted networking. This will allow the VM to connect to the internet.")
	rootCmd.PersistentFlags().Uint64Var(&vmMemAllocFlag, "vm-mem-alloc", 512, "Specifies the VM memory allocation in KiB")
}
