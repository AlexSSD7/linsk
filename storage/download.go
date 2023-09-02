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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

func (s *Storage) download(ctx context.Context, url string, hash []byte, out string, applyReaderMiddleware func(io.Reader) io.Reader) error {
	outClean := filepath.Clean(out)

	var created, success bool

	defer func() {
		if created && !success {
			_ = os.Remove(outClean)
		}
	}()

	_, err := os.Stat(outClean)
	if err == nil {
		return errors.Wrap(err, "file already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.Wrap(err, "stat out path")
	}

	f, err := os.OpenFile(outClean, os.O_CREATE|os.O_WRONLY, 0400)
	if err != nil {
		return errors.Wrap(err, "open file")
	}

	created = true

	defer func() { _ = f.Close() }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrap(err, "create new http get request")
	}

	s.logger.Info("Starting to download file", "from", url, "to", outClean)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "http get")
	}

	defer func() { _ = resp.Body.Close() }()

	knownSize := resp.ContentLength

	var readFrom io.Reader
	if applyReaderMiddleware != nil {
		readFrom = applyReaderMiddleware(resp.Body)
		knownSize = 0
	} else {
		readFrom = resp.Body
	}

	n, err := copyWithProgressAndHash(f, readFrom, 1024, hash, func(downloaded int) {
		var percent float64
		if knownSize != 0 {
			percent = float64(downloaded) / float64(knownSize)
		}

		lg := s.logger.With("out", outClean, "done", humanize.Bytes(uint64(downloaded)))
		if percent != 0 {
			lg.Info("Downloading file", "percent", math.Round(percent*100*100)/100)
		} else {
			lg.Info("Downloading compressed file", "percent", "N/A")
		}
	})
	if err != nil {
		return errors.Wrap(err, "copy resp to file")
	}

	s.logger.Info("Successfully downloaded file", "from", url, "to", outClean, "out-size", humanize.Bytes(uint64(n)))

	success = true

	return nil
}

func copyWithProgressAndHash(dst io.Writer, src io.Reader, blockSize int, wantHash []byte, report func(int)) (int, error) {
	block := make([]byte, blockSize)

	var h hash.Hash
	if wantHash != nil {
		h = sha256.New()
	}

	var progress int

	for {
		read, err := src.Read(block)
		if read > 0 {
			written, err := dst.Write(block[:read])
			if err != nil {
				return progress, errors.Wrap(err, "write")
			}

			if h != nil {
				h.Write(block[:read])
			}

			progress += written
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return progress, errors.Wrap(err, "read")
		}

		if progress%1000000 == 0 {
			report(progress)
		}
	}

	if h != nil {
		sum := h.Sum(nil)
		if !bytes.Equal(sum, wantHash) {
			return progress, fmt.Errorf("hash mismach: want '%v', have '%v'", hex.EncodeToString(wantHash), hex.EncodeToString(sum))
		}
	}

	return progress, nil
}
