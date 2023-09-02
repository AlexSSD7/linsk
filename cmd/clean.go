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
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/AlexSSD7/linsk/nettap"
	"github.com/AlexSSD7/linsk/utils"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove the Linsk data directory.",
	Run: func(cmd *cobra.Command, args []string) {
		store := createStoreOrExit()

		if nettap.Available() {
			tm, err := nettap.NewTapManager(slog.With("caller", "nettap-manager"))
			if err != nil {
				slog.Error("Failed to create network tap manager, will not attempt to remove dangling tap interfaces", "error", err.Error())
			} else {
				tapAllocs, err := store.ListNetTapAllocations()
				if err != nil {
					slog.Error("Failed to list net tap allocations, will not attempt to remove dangling tap interfaces", "error", err.Error())
				} else {
					removed, err := tm.PruneTaps(tapAllocs)
					if err != nil {
						slog.Error("Failed to prune dangling network taps", "error", err.Error())
					} else if len(removed) > 0 {
						slog.Info("Removed dangling network taps", "count", len(removed))
					}

					for _, removedTapName := range removed {
						err = store.ReleaseNetTapAllocation(removedTapName)
						if err != nil {
							slog.Error("Failed to release removed network tap allocation", "error", err.Error(), "name", removedTapName)
						}
					}
				}
			}
		}

		rmPath := store.DataDirPath()
		fmt.Fprintf(os.Stderr, "Will permanently remove '"+rmPath+"'. Proceed? (y/n) > ")

		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadBytes('\n')
		if err != nil {
			slog.Error("Failed to read answer", "error", err.Error())
			os.Exit(1)
		}

		if utils.ClearUnprintableChars(strings.ToLower(string(answer)), false) != "y" {
			fmt.Fprintf(os.Stderr, "Aborted.\n")
			os.Exit(2)
		}

		err = os.RemoveAll(rmPath)
		if err != nil {
			slog.Error("Failed to remove all", "error", err.Error(), "path", rmPath)
			os.Exit(1)
		}

		slog.Info("Deleted data directory", "path", rmPath)
	},
}
