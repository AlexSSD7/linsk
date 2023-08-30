package storage

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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
		err := s.download(constants.GetAlpineBaseImageURL(), constants.GetAlpineBaseImageHash(), baseImagePath)
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

func (s *Storage) BuildVMImageWithInterruptHandler(showBuilderVMDisplay bool, overwrite bool) error {
	var overwriting bool
	vmImagePath := s.GetVMImagePath()
	_, err := os.Stat(vmImagePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(err, "stat vm image path")
		}
	} else {
		if overwrite {
			overwriting = true
			err = os.Remove(vmImagePath)
			if err != nil {
				return errors.Wrap(err, "remove existing vm image")
			}
		} else {
			return ErrImageAlreadyExists
		}
	}

	baseImagePath, err := s.CheckDownloadBaseImage()
	if err != nil {
		return errors.Wrap(err, "check download base image")
	}

	s.logger.Info("Building VM image", "tags", constants.GetAlpineBaseImageTags(), "overwriting", overwriting, "dst", vmImagePath)

	buildCtx, err := imgbuilder.NewBuildContext(s.logger.With("subcaller", "imgbuilder"), baseImagePath, vmImagePath, showBuilderVMDisplay)
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
