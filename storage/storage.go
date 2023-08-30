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

var imageHash = [64]byte{70, 23, 243, 131, 146, 197, 41, 223, 67, 223, 41, 243, 128, 147, 82, 238, 34, 24, 123, 246, 251, 117, 120, 72, 72, 64, 96, 146, 227, 199, 49, 169, 164, 33, 205, 217, 98, 255, 109, 18, 130, 203, 126, 83, 34, 4, 229, 108, 173, 22, 107, 37, 181, 17, 84, 13, 129, 110, 25, 126, 158, 50, 135, 9}

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

func (s *Storage) GetLocalImagePath() string {
	return filepath.Join(s.path, hex.EncodeToString(imageHash[:])+".qcow2")
}

func (s *Storage) DownloadImage() error {
	localImagePath := s.GetLocalImagePath()

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

	s.logger.Info("Starting to download the VM image", "path", localImagePath)

	resp, err := http.Get(imageURL)
	if err != nil {
		return errors.Wrap(err, "http get image")
	}

	defer func() { _ = resp.Body.Close() }()

	_, err = copyWithProgress(f, resp.Body, 1024, resp.ContentLength, func(i int, f float64) {
		s.logger.Info("Downloading the VM image", "url", imageURL, "percent", math.Round(f*100*100)/100, "content-length", humanize.Bytes(uint64(resp.ContentLength)))
	})
	if err != nil {
		return errors.Wrap(err, "copy resp to file")
	}

	err = s.ValidateImageHash()
	if err != nil {
		return errors.Wrap(err, "validate image hash")
	}

	s.logger.Info("Successfully downloaded the VM image", "dst", localImagePath)

	success = true

	return nil
}

func (s *Storage) ValidateImageHash() error {
	localImagePath := s.GetLocalImagePath()

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

	s.logger.Info("Validated the VM image hash", "path", localImagePath)

	return nil
}

func (s *Storage) ValidateImageHashOrDownload() (bool, error) {
	err := s.ValidateImageHash()
	if err == nil {
		return false, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return true, errors.Wrap(s.DownloadImage(), "download image")
	}

	return false, err
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
