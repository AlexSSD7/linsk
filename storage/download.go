package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"math"
	"net/http"
	"os"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

func (s *Storage) download(url string, hash []byte, out string, applyReaderMiddleware func(io.Reader) io.Reader) error {
	var created, success bool

	defer func() {
		if created && !success {
			_ = os.Remove(out)
		}
	}()

	_, err := os.Stat(out)
	if err == nil {
		return errors.Wrap(err, "file already exists")
	} else {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(err, "stat out path")
		}
	}

	f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY, 0400)
	if err != nil {
		return errors.Wrap(err, "open file")
	}

	created = true

	defer func() { _ = f.Close() }()

	s.logger.Info("Starting to download file", "from", url, "to", out)

	resp, err := http.Get(url)
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

		lg := s.logger.With("out", out, "done", humanize.Bytes(uint64(downloaded)))
		if percent != 0 {
			lg.Info("Downloading file", "percent", math.Round(percent*100*100)/100)
		} else {
			lg.Info("Downloading compressed file", "percent", "N/A")
		}
	})
	if err != nil {
		return errors.Wrap(err, "copy resp to file")
	}

	s.logger.Info("Successfully downloaded file", "from", url, "to", out, "out-size", humanize.Bytes(uint64(n)))

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
