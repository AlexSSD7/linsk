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

package storage

import (
	"compress/bzip2"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/AlexSSD7/linsk/constants"
	"github.com/AlexSSD7/linsk/imgbuilder"
	"github.com/pkg/errors"
)

type Storage struct {
	logger *slog.Logger

	path string
}

func NewStorage(logger *slog.Logger, dataDir string) (*Storage, error) {
	dataDir = filepath.Clean(dataDir)

	err := os.MkdirAll(dataDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("mkdir all data dir")
	}

	return &Storage{
		logger: logger,

		path: dataDir,
	}, nil
}

func (s *Storage) CheckDownloadBaseImage(ctx context.Context) (string, error) {
	baseImagePath := filepath.Join(s.path, constants.GetAlpineBaseImageFileName())
	_, err := os.Stat(baseImagePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", errors.Wrap(err, "stat base image path")
		}

		// Image doesn't exist. Download one.
		err := s.download(ctx, constants.GetAlpineBaseImageURL(), constants.GetAlpineBaseImageHash(), baseImagePath, nil)
		if err != nil {
			return "", errors.Wrap(err, "download base alpine image")
		}

		return baseImagePath, nil
	}

	// Image exists. Ensure that the hash is correct.
	err = validateFileHash(baseImagePath, constants.GetAlpineBaseImageHash())
	if err != nil {
		return "", errors.Wrap(err, "validate hash of existing image")
	}

	return baseImagePath, nil
}

func (s *Storage) GetVMImagePath() string {
	return filepath.Join(s.path, constants.GetVMImageTags()+".qcow2")
}

func (s *Storage) GetAarch64EFIImagePath() string {
	return filepath.Join(s.path, constants.GetAarch64EFIImageName())
}

func (s *Storage) RunCLIImageBuild(showBuilderVMDisplay bool, overwrite bool) int {
	vmImagePath := s.GetVMImagePath()
	removed, err := checkExistsOrRemove(vmImagePath, overwrite)
	if err != nil {
		slog.Error("Failed to check for (or remove if overwrite mode is on) existing VM image", "error", err.Error())
		return 1
	}

	// We're using context.Background() everywhere because this is intended
	// to be executed as a blocking CLI command.

	baseImagePath, err := s.CheckDownloadBaseImage(context.Background())
	if err != nil {
		slog.Error("Failed to check or download base VM image", "error", err.Error())
		return 1
	}

	biosPath, err := s.CheckDownloadVMBIOS(context.Background())
	if err != nil {
		slog.Error("Failed to check or download VM BIOS", "error", err.Error())
		return 1
	}

	s.logger.Info("Building VM image", "tags", constants.GetAlpineBaseImageTags(), "overwriting", removed, "dst", vmImagePath)

	buildCtx, err := imgbuilder.NewBuildContext(s.logger.With("subcaller", "imgbuilder"), baseImagePath, vmImagePath, showBuilderVMDisplay, biosPath)
	if err != nil {
		slog.Error("Failed to create new image build context", "error", err.Error())
		return 1
	}

	exitCode := buildCtx.RunCLIBuild()
	if exitCode != 0 {
		return exitCode
	}

	err = os.Remove(baseImagePath)
	if err != nil {
		s.logger.Error("Failed to remove base image", "error", err.Error(), "path", baseImagePath)
	} else {
		s.logger.Info("Removed base image", "path", baseImagePath)
	}

	return 0
}

func (s *Storage) CheckVMImageExists() (string, error) {
	p := s.GetVMImagePath()
	_, err := os.Stat(p)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", errors.Wrap(err, "stat vm image path")
		}

		// Image doesn't exist.
		return "", nil
	}

	// Image exists. Returning the full path.
	return p, nil
}

func (s *Storage) DataDirPath() string {
	return s.path
}

func (s *Storage) CheckDownloadVMBIOS(ctx context.Context) (string, error) {
	if runtime.GOARCH == "arm64" {
		p, err := s.CheckDownloadAarch64EFIImage(ctx)
		if err != nil {
			return "", errors.Wrap(err, "check/download aarch64 efi image")
		}

		return p, nil
	}

	// On x86_64, there is no requirement to supply QEMU with any BIOS images.

	return "", nil
}

func (s *Storage) CheckDownloadAarch64EFIImage(ctx context.Context) (string, error) {
	efiImagePath := s.GetAarch64EFIImagePath()
	_, err := os.Stat(efiImagePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", errors.Wrap(err, "stat base image path")
		}

		// EFI image doesn't exist. Download one.
		err := s.download(ctx, constants.GetAarch64EFIImageBZ2URL(), constants.GetAarch64EFIImageHash(), efiImagePath, bzip2.NewReader)
		if err != nil {
			return "", errors.Wrap(err, "download base alpine image")
		}

		return efiImagePath, nil
	}

	// EFI image exists. Ensure that the hash is correct.
	err = validateFileHash(efiImagePath, constants.GetAarch64EFIImageHash())
	if err != nil {
		return "", errors.Wrap(err, "validate hash of existing image")
	}

	return efiImagePath, nil
}
