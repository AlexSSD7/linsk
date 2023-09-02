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
