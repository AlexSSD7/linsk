package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func validateFileHash(path string, hash []byte) error {
	pathClean := filepath.Clean(path)

	f, err := os.OpenFile(pathClean, os.O_RDONLY, 0400)
	if err != nil {
		return errors.Wrap(err, "open file")
	}

	defer func() { _ = f.Close() }()

	h := sha256.New()
	block := make([]byte, 1024)
	for {
		read, err := f.Read(block)
		if read > 0 {
			h.Write(block[:read])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return errors.Wrap(err, "read file block")
		}
	}

	sum := h.Sum(nil)

	if !bytes.Equal(sum, hash) {
		return fmt.Errorf("hash mismatch: want '%v', have '%v' (path '%v')", hex.EncodeToString(hash), hex.EncodeToString(sum), pathClean)
	}

	return nil
}
