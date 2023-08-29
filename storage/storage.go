package storage

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

const imageURL = "http://localhost:8000/linsk-base.qcow2"

var imageHash = [64]byte{96, 134, 26, 122, 43, 140, 212, 78, 44, 123, 103, 209, 21, 36, 81, 152, 9, 177, 47, 114, 225, 117, 64, 198, 50, 151, 71, 100, 1, 92, 106, 24, 224, 254, 157, 125, 188, 118, 84, 200, 47, 11, 215, 252, 100, 173, 64, 202, 132, 110, 15, 240, 234, 223, 56, 125, 94, 94, 179, 39, 193, 215, 41, 109}

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

func (s *Storage) getLocalImagePath() string {
	return filepath.Join(s.path, hex.EncodeToString(imageHash[:])+".qcow2")
}

func (s *Storage) DownloadImage() error {
	localImagePath := s.getLocalImagePath()

	var created, success bool

	defer func() {
		if created && !success {
			_ = os.Remove(localImagePath)
		}
	}()

	_, err := os.Stat(localImagePath)
	if err == nil {
		err = os.Remove(localImagePath)
		if err != nil {
			return errors.Wrap(err, "remove existing image")
		}
	} else {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(err, "stat local image path")
		}
	}

	f, err := os.OpenFile(localImagePath, os.O_CREATE|os.O_WRONLY, 0400)
	if err != nil {
		return errors.Wrap(err, "open file")
	}

	created = true

	defer func() { _ = f.Close() }()

	resp, err := http.Get(imageURL)
	if err != nil {
		return errors.Wrap(err, "http get image")
	}

	defer func() { _ = resp.Body.Close() }()

	_, err = copyWithProgress(f, resp.Body, 1024, resp.ContentLength, func(i int, f float64) {
		s.logger.Info("Downloading image", "url", imageURL, "percent", math.Round(f*100*100)/100, "content-length", humanize.Bytes(uint64(resp.ContentLength)))
	})
	if err != nil {
		return errors.Wrap(err, "copy resp to file")
	}

	err = s.ValidateImageHash()
	if err != nil {
		return errors.Wrap(err, "validate image hash")
	}

	success = true

	return nil
}

func (s *Storage) ValidateImageHash() error {
	localImagePath := s.getLocalImagePath()

	f, err := os.OpenFile(localImagePath, os.O_RDONLY, 0400)
	if err != nil {
		return errors.Wrap(err, "open file")
	}

	defer func() { _ = f.Close() }()

	h := sha512.New()
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

	if !bytes.Equal(sum, imageHash[:]) {
		return fmt.Errorf("hash mismatch: want '%v', have '%v'", hex.EncodeToString(imageHash[:]), hex.EncodeToString(sum))
	}

	return nil
}

func copyWithProgress(dst io.Writer, src io.Reader, blockSize int, length int64, report func(int, float64)) (int, error) {
	block := make([]byte, blockSize)

	var progress int

	for {
		read, err := src.Read(block)
		if read > 0 {
			written, err := dst.Write(block[:read])
			if err != nil {
				return progress, errors.Wrap(err, "write")
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
			var percent float64
			if length != 0 {
				percent = float64(progress) / float64(length)
			}
			report(progress, percent)
		}
	}

	return progress, nil
}
