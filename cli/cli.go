package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jvatic/audible-downloader/audible"
	"github.com/jvatic/audible-downloader/internal/common"
	"github.com/jvatic/audible-downloader/internal/config"
	"github.com/jvatic/audible-downloader/internal/downloader"
	"github.com/jvatic/audible-downloader/internal/ffmpeg"
	"github.com/jvatic/audible-downloader/internal/prompt"
	"github.com/jvatic/audible-downloader/internal/utils"
	log "github.com/sirupsen/logrus"
	mpb "github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("Error: %s", err)
	}

	ctx := common.InitShutdownSignals(context.Background())

	cli := &CLI{}

	if err := cli.Run(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

type CLI struct {
	DstDir string
}

func (cli *CLI) Run(runCtx context.Context) error {
	// Ask user which Audible region/domain to use
	region := RegionPrompt()
	u, err := url.Parse(fmt.Sprintf("https://www.audible.%s", region.TLD))
	if err != nil {
		log.Fatalf("Unable to parse domain: %v", err)
	}
	log.Infof("Using %s (%s)\n", u.Host, region.Name)

	var username string
	var password string
	for {
		username = PromptString("Audible Username")
		if username == "" {
			log.Error("Audible Username required")
			continue
		}
		break
	}
	for {
		password = PromptPassword("Audible Password")
		if password == "" {
			log.Error("Audible Password required")
			continue
		}
		break
	}

	c, err := audible.NewClient(
		audible.OptionBaseURL(u.String()),
		audible.OptionUsername(username),
		audible.OptionPassword(password),
		audible.OptionCaptcha(PromptCaptcha),
		audible.OptionAuthCode(func() string {
			return PromptString("Auth Code (OTP)")
		}),
		audible.OptionPromptChoice(func(msg string, opts []string) int {
			return PromptChoice(msg, opts)
		}),
	)
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	if err := c.Authenticate(utils.ContextWithCancelChan(context.Background(), runCtx.Done())); err != nil {
		return fmt.Errorf("error authenticating: %w", err)
	}

	if err := cli.DownloadLibrary(
		utils.ContextWithCancelChan(context.Background(), runCtx.Done()),
		c,
	); err != nil {
		return fmt.Errorf("error downloading library: %w", err)
	}

	return nil
}

func (cli *CLI) GetNewBooks(books []*audible.Book) ([]*audible.Book, []*audible.Book, error) {
	localBooks, err := common.ListDownloadedBooks(cli.DstDir)
	if err != nil {
		return nil, nil, fmt.Errorf("GetNewBooks Error: Unable to enumerate downloaded books: %w", err)
	}
	localBooksByID := make(map[string]*audible.Book, len(localBooks))
	for _, b := range localBooks {
		localBooksByID[b.ID()] = b
	}

	newBooks := make([]*audible.Book, 0)
	downloadedBooks := make([]*audible.Book, 0, len(localBooks))
	for _, b := range books {
		if dlb, ok := localBooksByID[b.ID()]; ok {
			b.LocalPath = dlb.LocalPath
			if filepath.Ext(dlb.LocalPath) == ".mp4" {
				// we're assuming any .mp4 found is completely downloaded
				downloadedBooks = append(downloadedBooks, b)
				continue
			}
			// the book may be partially downloaded
		}
		newBooks = append(newBooks, b)
	}
	return newBooks, downloadedBooks, nil
}

func (cli *CLI) DownloadLibrary(ctx context.Context, c *audible.Client) error {
	activationBytes, err := c.GetActivationBytes(ctx)
	if err != nil {
		return fmt.Errorf("error getting activation bytes: %s", err)
	}
	log.Debugf("Activation Bytes: %s\n", string(activationBytes))

	books, err := c.GetLibrary(ctx)
	if err != nil {
		return fmt.Errorf("error reading library: %s", err)
	}

	cli.DstDir = PromptDownloadDir()

	var downloadedBooks []*audible.Book
	books, downloadedBooks, err = cli.GetNewBooks(books)
	if err != nil {
		return fmt.Errorf("error filtering library: %w", err)
	}

	// write info.txt file for all books already downloaded
	for _, b := range downloadedBooks {
		dir := filepath.Dir(b.LocalPath)
		fi, err := os.Lstat(dir)
		if err == nil && fi.IsDir() {
			// book exists
			if err := common.WriteInfoFile(filepath.Dir(b.LocalPath), b); err != nil {
				fmt.Printf("Error writing info file for %q: %s\n", b.Title, err)
			}
		}
	}

	if len(books) == 0 {
		log.Info("You have downloaded all the books from your Audible library.")
		return nil
	}

	getDstPath := PromptPathTemplate()
	for _, b := range books {
		p := filepath.Join(cli.DstDir, getDstPath(b))
		if b.LocalPath == "" {
			b.LocalPath = p
		}
	}

loop:
	for {
		answer := strings.ToLower(prompt.String(fmt.Sprintf("Download %d new books from your Audible library? (yes/no/list)", len(books)), prompt.Required))
		switch answer {
		case "yes":
			break loop
		case "list":
			for i, b := range books {
				fmt.Printf("%02d) %s by %s\n", i+1, b.Title, strings.Join(b.Authors, ", "))
			}
			break loop
		default:
			return fmt.Errorf("download aborted")
		}
	}

	pbgroup := mpb.New()
	mainBar := pbgroup.AddBar(int64(len(books)),
		mpb.BarPriority(0),
		mpb.PrependDecorators(
			decor.Name("Books", decor.WCSyncSpaceR),
			decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
		),
	)

	logFile, err := os.OpenFile("output.log", os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer logFile.Close()

	dlm, err := downloader.NewDownloader(downloader.OptionLogWriter(logFile))
	if err != nil {
		return fmt.Errorf("error initializing downloader: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(len(books))

	client := &http.Client{
		Jar: c.Jar,
	}

	var errs []error
	var errsMtx sync.Mutex

	pushError := func(err error) {
		fmt.Fprintln(logFile, err)
		errsMtx.Lock()
		defer errsMtx.Unlock()
		errs = append(errs, err)
	}

	downloadBook := func(book *audible.Book) {
		defer wg.Done()
		defer mainBar.Increment()

		dir := filepath.Dir(book.LocalPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			pushError(fmt.Errorf("error creating dir for %q: %s", book.Title, err))
			return
		}

		if err := common.WriteInfoFile(filepath.Dir(book.LocalPath), book); err != nil {
			pushError(fmt.Errorf("error writing info file for %q: %s", book.Title, err))
		}

		var bookwg sync.WaitGroup
		for _, u := range book.DownloadURLs {
			bookwg.Add(1)
			go func(u string) {
				defer bookwg.Done()

				var bar *mpb.Bar

				// start the download
				dlTime := time.Now()
				var dl *downloader.Download
				dl = downloader.NewDownload(
					downloader.DownloadOptionDirname(dir),
					downloader.DownloadOptionURL(u),
					downloader.DownloadOptionDetectFilename(),
					downloader.DownloadOptionHTTPClient(client),
					downloader.DownloadOptionProgress(func(totalBytes int64, completedBytes int64) {
						if bar == nil {
							bar = pbgroup.AddBar(totalBytes,
								mpb.BarRemoveOnComplete(),
								mpb.PrependDecorators(
									decor.Name(filepath.Base(dl.OutputPath()), decor.WCSyncSpaceR),
									decor.Name("downloading", decor.WCSyncSpaceR),
									decor.Counters(decor.UnitKB, "% .1f / % .1f"),
								),
								mpb.AppendDecorators(
									decor.EwmaETA(decor.ET_STYLE_MMSS, 0, decor.WCSyncWidth),
								),
							)
						}
						bar.SetCurrent(completedBytes)
						bar.DecoratorEwmaUpdate(time.Since(dlTime))
						dlTime = time.Now()
					}),
				)
				dlm.Add(dl)

				// wait for it to complete
				if err := dl.Wait(); err != nil {
					if err == downloader.ErrAbort {
						fmt.Fprintf(logFile, "Skipping %s\n", dl.OutputPath())
						return
					}
					pushError(fmt.Errorf("error downloading %s: %s", u, err))
				}

				outPath := dl.OutputPath()

				// decrypt if it's an .aax file
				if filepath.Ext(outPath) == ".aax" {
					bar = nil
					if err := ffmpeg.DecryptAudioBook(
						ffmpeg.InputPath(outPath),
						ffmpeg.OutputPath(utils.SwapFileExt(outPath, ".mp4")),
						ffmpeg.ActivationBytes(string(activationBytes)),
						ffmpeg.Progress(func(totalBytes int64, completedBytes int64) {
							if bar == nil {
								bar = pbgroup.AddBar(totalBytes,
									mpb.BarRemoveOnComplete(),
									mpb.PrependDecorators(
										decor.Name(book.Title, decor.WCSyncSpaceR),
										decor.Name("decrypting", decor.WCSyncSpaceR),
									),
									mpb.AppendDecorators(
										decor.Percentage(decor.WC{W: 5}),
									),
								)
							}
							bar.SetCurrent(completedBytes)
						}),
					); err != nil {
						pushError(fmt.Errorf("error decrypting %s: %s", u, err))
					}

					// remove original .aax file
					if err := os.Remove(outPath); err != nil {
						pushError(fmt.Errorf("error removing .aax file for %q: %s", book.Title, err))
					}
				}
			}(u)
		}
		bookwg.Wait()
	}

	dlm.Start()
	for _, book := range books {
		go downloadBook(book)
	}

	wg.Wait()
	dlm.Wait()
	pbgroup.Wait()

	errsMtx.Lock()
	for _, err := range errs {
		fmt.Println(err)
	}
	errsMtx.Unlock()

	return nil
}
