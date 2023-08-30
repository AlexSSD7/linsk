package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/AlexSSD7/linsk/vm"
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start a VM and expose an FTP file share.",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		vmMountDevName := args[1]
		fsType := args[2]

		ftpPassivePortCount := uint16(9)

		networkSharePort, err := getClosestAvailPortWithSubsequent(9000, 10)
		if err != nil {
			slog.Error("Failed to get closest available host port for network file share", "error", err.Error())
			os.Exit(1)
		}

		ftpListenIP := net.ParseIP(ftpListenAddrFlag)
		if ftpListenIP == nil {
			slog.Error("Invalid FTP listen address specified", "value", ftpListenAddrFlag)
			os.Exit(1)
		}

		ftpExtIP := net.ParseIP(ftpExtIPFlag)
		if ftpExtIP == nil {
			slog.Error("Invalid FTP external IP specified", "value", ftpExtIPFlag)
			os.Exit(1)
		}

		if ftpListenAddrFlag != defaultFTPListenAddr && ftpExtIPFlag == defaultFTPListenAddr {
			slog.Warn("No external FTP IP address via --ftp-extip was configured. This is a requirement in almost all scenarios if you want to connect remotely.")
		}

		ports := []vm.PortForwardingRule{{
			HostIP:   ftpListenIP,
			HostPort: networkSharePort,
			VMPort:   21,
		}}

		for i := uint16(0); i < ftpPassivePortCount; i++ {
			p := networkSharePort + 1 + i
			ports = append(ports, vm.PortForwardingRule{
				HostIP:   ftpListenIP,
				HostPort: p,
				VMPort:   p,
			})
		}

		os.Exit(runVM(args[0], func(ctx context.Context, i *vm.VM, fm *vm.FileManager) int {
			slog.Info("Mounting the device", "dev", vmMountDevName, "fs", fsType, "luks", luksFlag)

			err := fm.Mount(vmMountDevName, vm.MountOptions{
				FSType: fsType,
				LUKS:   luksFlag,
			})
			if err != nil {
				slog.Error("Failed to mount the disk inside the VM", "error", err.Error())
				return 1
			}

			sharePWD, err := password.Generate(16, 10, 0, false, false)
			if err != nil {
				slog.Error("Failed to generate ephemeral password for the network file share", "error", err.Error())
				return 1
			}

			err = fm.StartFTP(sharePWD, networkSharePort+1, ftpPassivePortCount, ftpExtIP)
			if err != nil {
				slog.Error("Failed to start FTP server", "error", err.Error())
				return 1
			}

			slog.Info("Started the network share successfully", "type", "ftp")

			shareURI := "ftp://linsk:" + sharePWD + "@" + ftpExtIP.String() + ":" + fmt.Sprint(networkSharePort)

			fmt.Fprintf(os.Stderr, "================\n[Network File Share Config]\nThe network file share was started. Please use the credentials below to connect to the file server.\n\nType: FTP\nServer Address: ftp://%v:%v\nUsername: linsk\nPassword: %v\n\nShare URI: %v\n================\n", ftpExtIP.String(), networkSharePort, sharePWD, shareURI)

			<-ctx.Done()
			return 0
		}, ports, unrestrictedNetworkingFlag))
	},
}

var luksFlag bool
var ftpListenAddrFlag string
var ftpExtIPFlag string

const defaultFTPListenAddr = "127.0.0.1"

func init() {
	runCmd.Flags().BoolVarP(&luksFlag, "luks", "l", false, "Use cryptsetup to open a LUKS volume (password will be prompted).")
	runCmd.Flags().StringVar(&ftpListenAddrFlag, "ftp-listen", defaultFTPListenAddr, "Specifies the address to bind the FTP ports to. NOTE: Changing bind address is not enough to connect remotely. You should also specify --ftp-extip.")
	runCmd.Flags().StringVar(&ftpExtIPFlag, "ftp-extip", defaultFTPListenAddr, "Specifies the external IP the FTP server should advertise.")
}
