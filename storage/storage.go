package storage

import (
	"compress/bzip2"
	"fmt"
	"io"
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

func (s *Storage) CheckDownloadBaseImage() (string, error) {
	baseImagePath := filepath.Join(s.path, constants.GetAlpineBaseImageFileName())
	_, err := os.Stat(baseImagePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", errors.Wrap(err, "stat base image path")
		}

		// Image doesn't exist. Download one.
		err := s.download(constants.GetAlpineBaseImageURL(), constants.GetAlpineBaseImageHash(), baseImagePath, nil)
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

func (s *Storage) BuildVMImageWithInterruptHandler(showBuilderVMDisplay bool, overwrite bool) error {
	vmImagePath := s.GetVMImagePath()
	removed, err := checkExistsOrRemove(vmImagePath, overwrite)
	if err != nil {
		return errors.Wrap(err, "check exists or remove")
	}

	baseImagePath, err := s.CheckDownloadBaseImage()
	if err != nil {
		return errors.Wrap(err, "check/download base image")
	}

	biosPath, err := s.CheckDownloadCPUArchSpecifics()
	if err != nil {
		return errors.Wrap(err, "check/download cpu arch specifics")
	}

	s.logger.Info("Building VM image", "tags", constants.GetAlpineBaseImageTags(), "overwriting", removed, "dst", vmImagePath)

	buildCtx, err := imgbuilder.NewBuildContext(s.logger.With("subcaller", "imgbuilder"), baseImagePath, vmImagePath, showBuilderVMDisplay, biosPath)
	if err != nil {
		return errors.Wrap(err, "create new img build context")
	}

	return errors.Wrap(buildCtx.BuildWithInterruptHandler(), "build")
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

func (s *Storage) CheckDownloadCPUArchSpecifics() (string, error) {
	if runtime.GOARCH == "arm64" {
		p, err := s.CheckDownloadAarch64EFIImage()
		if err != nil {
			return "", errors.Wrap(err, "check/download aarch64 efi image")
		}

		return p, nil
	}

	return "", nil
}

func (s *Storage) CheckDownloadAarch64EFIImage() (string, error) {
	efiImagePath := s.GetAarch64EFIImagePath()
	_, err := os.Stat(efiImagePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", errors.Wrap(err, "stat base image path")
		}

		// EFI image doesn't exist. Download one.
		err := s.download(constants.GetAarch64EFIImageBZ2URL(), constants.GetAarch64EFIImageHash(), efiImagePath, func(r io.Reader) io.Reader {
			return bzip2.NewReader(r)
		})
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
