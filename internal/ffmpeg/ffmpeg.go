package ffmpeg

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jvatic/audible-downloader/internal/utils"
)

type ProgressFunc = func(totalBytes int64, completedBytes int64)

type decrypter struct {
	inPath          string
	outPath         string
	activationBytes string
	progressHook    ProgressFunc
}

type option func(d *decrypter)

func InputPath(inPath string) option {
	return func(d *decrypter) {
		d.inPath = inPath
	}
}

func OutputPath(outPath string) option {
	return func(d *decrypter) {
		d.outPath = outPath
	}
}

func ActivationBytes(activationBytes string) option {
	return func(d *decrypter) {
		d.activationBytes = activationBytes
	}
}

func Progress(progressHook ProgressFunc) option {
	return func(d *decrypter) {
		d.progressHook = progressHook
	}
}

func DecryptAudioBook(opts ...option) error {
	d := &decrypter{}
	for _, opt := range opts {
		opt(d)
	}

	filename := filepath.Base(d.inPath)
	outPath := filepath.Base(d.outPath)
	if outPath == "" {
		outPath = utils.SwapFileExt(filename, ".mp4")
	}

	if _, err := os.Lstat(outPath); err == nil {
		// already done
		return nil
	}

	fi, err := os.Lstat(d.inPath)
	if err != nil {
		// input file doesn't exist or can't be read
		return err
	}
	inputSize := fi.Size()

	extractCoverPhoto(d.inPath)

	s, err := startProgressServer(outPath, inputSize, d.progressHook)
	if err != nil {
		return err
	}
	defer s.Close()

	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg", "-y", "-activation_bytes", d.activationBytes, "-progress", fmt.Sprintf("http://%s", s.Addr().String()), "-i", filename, "-vn", "-c:a", "copy", outPath)
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = &stderr
	cmd.Dir = filepath.Dir(d.inPath)
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stderr, &stderr)
		return err
	}

	if d.progressHook != nil {
		// the output size is smaller than the input size, this reports it as completed
		d.progressHook(inputSize, inputSize)
	}

	return nil
}

func extractCoverPhoto(inPath string) (string, error) {
	outPath := filepath.Base(filepath.Join(filepath.Dir(inPath), "cover.png"))
	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg", "-y", "-i", filepath.Base(inPath), outPath)
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = &stderr
	cmd.Dir = filepath.Dir(inPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Error extracting cover photo: %v", err)
	}
	return outPath, nil
}

type progressServer struct {
	listener net.Listener
}

func startProgressServer(filename string, inputSize int64, progressHook ProgressFunc) (*progressServer, error) {
	s := &progressServer{}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s.listener = l

	if progressHook != nil {
		progressHook(inputSize, 0)
	}

	go func() {
		http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer req.Body.Close()

			s := bufio.NewScanner(req.Body)
			for s.Scan() {
				line := s.Text()
				if !strings.HasPrefix(line, "total_size=") {
					// we only care about the total bytes written
					continue
				}
				size, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "total_size=")), 10, 64)
				if err != nil {
					fmt.Printf("Error parsing line: %s\n", err)
				}

				if progressHook != nil {
					progressHook(inputSize, size)
				}
			}
		}))
	}()

	return s, nil
}

func (s *progressServer) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *progressServer) Close() error {
	return s.listener.Close()
}
