package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlexSSD7/linsk/nettap"
	"github.com/pkg/errors"
)

const tapAllocPrefix = "tap_alloc_"

func (s *Storage) getAllocFilePath(tapName string) (string, error) {
	err := nettap.ValidateTapName(tapName)
	if err != nil {
		return "", errors.Wrap(err, "validate tap name")
	}

	return filepath.Join(s.path, tapAllocPrefix+tapName), nil
}

func (s *Storage) SaveNetTapAllocation(tapName string, pid int) error {
	allocFilePath, err := s.getAllocFilePath(tapName)
	if err != nil {
		return errors.Wrap(err, "get alloc file path")
	}

	err = os.WriteFile(allocFilePath, []byte(fmt.Sprint(pid)), 0400)
	if err != nil {
		return errors.Wrap(err, "write alloc file")
	}

	return nil
}

func (s *Storage) ReleaseNetTapAllocation(tapName string) error {
	allocFilePath, err := s.getAllocFilePath(tapName)
	if err != nil {
		return errors.Wrap(err, "get alloc file path")
	}

	err = os.Remove(allocFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.logger.Warn("Attempted to remove non-existent tap allocation", "tap-name", tapName)
			return nil
		}

		return errors.Wrap(err, "remove alloc file")
	}

	return nil
}

func (s *Storage) ListNetTapAllocations() ([]nettap.Alloc, error) {
	dirEntries, err := os.ReadDir(s.path)
	if err != nil {
		return nil, errors.Wrap(err, "read data dir")
	}

	var ret []nettap.Alloc

	for _, entry := range dirEntries {
		if strings.HasPrefix(entry.Name(), tapAllocPrefix) {
			entryPath := filepath.Join(s.path, entry.Name())

			tapName := strings.TrimPrefix(entry.Name(), tapAllocPrefix)
			err := nettap.ValidateTapName(tapName)
			if err != nil {
				s.logger.Error("Failed to validate network tap name in tap allocation file, skipping. External interference?", "error", err.Error(), "name", tapName, "path", entryPath)
				continue
			}

			data, err := os.ReadFile(entryPath)
			if err != nil {
				return nil, errors.Wrapf(err, "read tap alloc file '%v'", entryPath)
			}

			pid, err := strconv.ParseUint(string(data), 10, strconv.IntSize-1) // We're aiming for `int` PID.
			if err != nil {
				return nil, errors.Wrapf(err, "parse pid (alloc file '%v')", entryPath)
			}

			ret = append(ret, nettap.Alloc{
				TapName: tapName,
				PID:     int(pid),
			})
		}
	}

	return ret, nil
}
