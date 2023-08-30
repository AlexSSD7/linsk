package storage

import (
	"os"

	"github.com/pkg/errors"
)

func checkExistsOrRemove(path string, overwriteRemove bool) (bool, error) {
	var removed bool

	_, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return removed, errors.Wrap(err, "stat file")
		}
	} else {
		if overwriteRemove {
			err = os.Remove(path)
			if err != nil {
				return removed, errors.Wrap(err, "remove file")
			}
			removed = true
		} else {
			return removed, ErrImageAlreadyExists
		}
	}

	return removed, nil
}
