package downloader

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/jvatic/audible-downloader/internal/utils"
)

type Option func(*Downloader)

func OptionPoolSize(size int) Option {
	return func(d *Downloader) {
		d.poolSize = size
	}
}

func OptionMaxRetries(n int) Option {
	return func(d *Downloader) {
		d.maxRetries = n
	}
}

func OptionLogWriter(w io.Writer) Option {
	return func(d *Downloader) {
		d.logWriter = w
	}
}

type ProgressFunc = func(totalBytes int64, completedBytes int64)

func NewDownloader(opts ...Option) (*Downloader, error) {
	d := &Downloader{
		poolSize:   5,
		maxRetries: 3,
		downloadCh: make(chan *Download),
		doneCh:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(d)
	}

	if d.logWriter == nil {
		d.logWriter = os.Stderr
	}

	return d, nil
}

type Downloader struct {
	poolSize   int
	maxRetries int
	downloadCh chan *Download
	doneCh     chan struct{}
	wg         sync.WaitGroup
	logWriter  io.Writer
}

// Start starts the download process.
func (d *Downloader) Start() {
	// add an item to the WaitGroup to wait for Wait() to be called
	d.wg.Add(1)

	go d.start()
}

// Wait is to be called when all downloads have been added and waits for them
// all to complete before returning.
func (d *Downloader) Wait() {
	d.wg.Done()
	<-d.doneCh
}

// Add adds a Download to the queue and may be called before or after Start,
// and before Wait.
func (d *Downloader) Add(dl *Download) {
	if d.logWriter == os.Stderr {
		dl.logWriter = d.logWriter
	}
	d.wg.Add(1)
	go func() {
		d.downloadCh <- dl
	}()
}

func (d *Downloader) start() {
	limiter := make(chan struct{}, d.poolSize)
	go func() {
		for {
			select {
			case download, ok := <-d.downloadCh:
				if !ok {
					return
				}
				go func(download *Download) {
					limiter <- struct{}{}
					download.doWithRetry(d.maxRetries)
					d.wg.Done()
					<-limiter
				}(download)
			case <-d.doneCh:
				return
			}
		}
	}()
	d.wg.Wait()
	close(d.doneCh)
}

type DownloadOption func(dl *Download)

func DownloadOptionDirname(dirname string) DownloadOption {
	return func(dl *Download) {
		dl.dirname = dirname
	}
}

func DownloadOptionDetectFilename() DownloadOption {
	return func(dl *Download) {
		dl.shouldDetectFilename = true
	}
}

func DownloadOptionURL(url string) DownloadOption {
	return func(dl *Download) {
		dl.url = url
	}
}

func DownloadOptionHTTPClient(client *http.Client) DownloadOption {
	return func(dl *Download) {
		dl.client = client
	}
}

func DownloadOptionFinalExt(ext string) DownloadOption {
	return func(dl *Download) {
		dl.finalExt = ext
	}
}

func DownloadOptionProgress(fn ProgressFunc) DownloadOption {
	return func(dl *Download) {
		dl.progressHook = fn
	}
}

func DownloadOptionLogWriter(w io.Writer) DownloadOption {
	return func(dl *Download) {
		dl.logWriter = w
	}
}

func DownloadOptionFilter(fn func(dl *Download) bool) DownloadOption {
	return func(dl *Download) {
		dl.filterFn = fn
	}
}

func NewDownload(opts ...DownloadOption) *Download {
	dl := &Download{
		doneCh: make(chan error),
	}

	for _, opt := range opts {
		opt(dl)
	}

	if dl.client == nil {
		dl.client = &http.Client{}
	}

	if dl.logWriter == nil {
		dl.logWriter = os.Stderr
	}

	return dl
}

type Download struct {
	dirname              string
	filename             string
	finalExt             string
	shouldDetectFilename bool
	url                  string
	client               *http.Client
	progressHook         ProgressFunc
	totalSize            int64
	doneCh               chan error
	logWriter            io.Writer
	filterFn             func(dl *Download) bool
}

func (dl *Download) OutputPath() string {
	return filepath.Join(dl.dirname, dl.filename)
}

func (dl *Download) Wait() error {
	err, _ := <-dl.doneCh
	return err
}

func (dl *Download) detectFilename() (string, error) {
	resp, err := dl.client.Head(dl.url)
	if err != nil {
		return "", fmt.Errorf("Error reading %s: %s\n", dl.url, err)
	}

	// update the url to be at the end of any redirects
	dl.url = resp.Request.URL.String()

	if fn, ok := utils.ParseHeaderLabels(resp.Header.Get("Content-Disposition"))["filename"]; ok {
		return utils.NormalizeFilename(fn), nil
	}
	return utils.NormalizeFilename(filepath.Base(resp.Request.URL.Path)), nil
}

type fileWriter struct {
	io.Writer
	progressCh chan int64
	closeHook  func()
}

func (fw *fileWriter) Write(b []byte) (int, error) {
	n, err := fw.Writer.Write(b)
	if fw.progressCh != nil {
		fw.progressCh <- int64(n)
	}
	return n, err
}

func (fw *fileWriter) Close() error {
	var err error
	if c, ok := fw.Writer.(io.Closer); ok {
		err = c.Close()
	}
	fw.closeHook()
	return err
}

func (dl *Download) getFileInfo() (os.FileInfo, error) {
	path := filepath.Join(dl.dirname, dl.filename)
	if fi, err := os.Lstat(path + ".part"); err == nil {
		return fi, nil
	}
	return os.Lstat(path)
}

func (dl *Download) setTotalSize(totalSize int64) {
	dl.totalSize = totalSize
}

func (dl *Download) getWriter() (io.Writer, error) {
	if err := os.MkdirAll(dl.dirname, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dl.dirname, dl.filename)
	file, err := os.OpenFile(path+".part", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, err
	}
	progressCh := make(chan int64)
	go func() {
		var nCompleted int64
		for {
			if n, ok := <-progressCh; ok {
				nCompleted += n
				if dl.progressHook != nil {
					dl.progressHook(dl.totalSize, nCompleted)
				}
			} else {
				break
			}
		}
	}()
	return &fileWriter{
		Writer:     file,
		progressCh: progressCh,
		closeHook: func() {
			if err := os.Rename(path+".part", path); err != nil {
				fmt.Fprintf(dl.logWriter, "%s failed to rename file\n", path)
			}
		},
	}, nil
}

var ErrAbort = errors.New("Download aborted")

func (dl *Download) doWithRetry(maxRetries int) {
	u, err := url.Parse(dl.url)
	if err != nil {
		dl.doneCh <- fmt.Errorf("Valid URL is required")
		return
	}

	if dl.dirname == "" {
		dl.doneCh <- fmt.Errorf("Dirname is required")
		return
	}

	if dl.shouldDetectFilename {
		dl.filename, err = dl.detectFilename()
		if err != nil {
			dl.doneCh <- err
			return
		}
	}

	if dl.filename == "" {
		dl.filename = utils.NormalizeFilename(filepath.Base(u.Path))
	}

	if dl.finalExt != "" {
		// check if the file already exists with a different ext
		// and assume the download is complete if it does
		if _, err := os.Lstat(utils.SwapFileExt(dl.OutputPath(), dl.finalExt)); err == nil {
			dl.doneCh <- nil
			return
		}
	}

	if dl.filterFn != nil {
		if !dl.filterFn(dl) {
			fmt.Fprintf(dl.logWriter, "Skipping %s", dl.OutputPath())
			dl.doneCh <- ErrAbort
			return
		}
	}

	for i := 1; i < maxRetries; i++ {
		if dlErr := dl.do(); dlErr != nil {
			err = fmt.Errorf("Error downloading file(%s): %s", filepath.Join(dl.dirname, dl.filename), dlErr)
			continue
		}
		err = nil
		break
	}
	dl.doneCh <- err
}

func (dl *Download) do() error {
	resp, err := dl.client.Get(dl.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	totalSize := resp.ContentLength
	var reqOffset int64

	fi, err := dl.getFileInfo()
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			// file exists, error reading info
			return err
		}
	}
	if err == nil && fi.Size() == totalSize {
		// file already downloaded
		if dl.progressHook != nil {
			dl.progressHook(totalSize, totalSize)
		}
		return nil
	}

	// used for progress reporting
	dl.setTotalSize(totalSize)

	var w io.Writer
	// Continue download if it's already been started (i.e. the file already
	// exists and is smaller than it should be)
	if err == nil && fi.Size() < totalSize {
		ar := resp.Header.Get("Accept-Ranges")
		if ar != "" && ar != "none" {
			req, err := http.NewRequest("GET", dl.url, nil)
			if err == nil {
				reqOffset = fi.Size() + 1
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-", reqOffset))
				resp, err = dl.client.Do(req)
				if err != nil {
					return fmt.Errorf("Do Request Error: %s", err)
				}

				if resp.ContentLength == (totalSize - reqOffset) {
					w, err = dl.getWriter()
					if err != nil {
						return err
					}
				} else if resp.ContentLength != totalSize {
					return fmt.Errorf("%s: Range response size mismatch: fi.Size: %d, reqOffset: %d, ContentLength: %d, totalSize: %d", dl.filename, fi.Size(), reqOffset, resp.ContentLength, totalSize)
				}
			} else {
				return fmt.Errorf("%s: Error creating range request: %s\n", dl.filename, err)
			}
		}
	}

	if w == nil {
		w, err = dl.getWriter()
		if err != nil {
			return err
		}
	}

	if closer, ok := w.(io.Closer); ok {
		defer closer.Close()
	}

	if dl.progressHook != nil {
		dl.progressHook(totalSize, reqOffset)
	}

	_, err = io.Copy(w, resp.Body)
	return err
}
