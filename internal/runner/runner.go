package runner

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jvatic/audible-downloader/audible"
	"github.com/jvatic/audible-downloader/internal/downloader"
	"github.com/jvatic/audible-downloader/internal/ffmpeg"
	"github.com/jvatic/audible-downloader/internal/prompt"
	"github.com/jvatic/audible-downloader/internal/utils"
	mpb "github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

func Run(runCtx context.Context) error {
	// Ask user which Audible region/domain to use
	region := RegionPrompt()
	u, err := url.Parse(fmt.Sprintf("https://www.audible.%s", region.TLD))
	if err != nil {
		log.Fatalf("Unable to parse domain: %v", err)
	}
	fmt.Printf("Using %s (%s)\n", u.Host, region.Name)

	username := prompt.String("Audible Username", prompt.Required)
	password := prompt.Password("Audible Password", prompt.Required)
	c, err := audible.NewClient(
		audible.OptionBaseURL(u.String()),
		audible.OptionUsername(username),
		audible.OptionPassword(password),
		audible.OptionCaptcha(func(imgURL string) string {
			return prompt.String(fmt.Sprintf("%s\nPlease enter captcha from above URL", imgURL), prompt.Required)
		}),
		audible.OptionAuthCode(func() string {
			return prompt.String("Auth Code", prompt.Required)
		}),
		audible.OptionRadioPrompt(func(msg string, opts []string) int {
			return prompt.Radio(msg, opts)
		}),
	)
	if err != nil {
		return fmt.Errorf("Error creating client: %w\n", err)
	}

	if err := c.Authenticate(utils.ContextWithCancelChan(context.Background(), runCtx.Done())); err != nil {
		return fmt.Errorf("Error authenticating: %w\n", err)
	}

	if err := DownloadLibrary(utils.ContextWithCancelChan(context.Background(), runCtx.Done()), c); err != nil {
		return fmt.Errorf("Error downloading library: %w", err)
	}

	return nil
}

func RegionPrompt() audible.Region {
	choices := make([]string, len(audible.Regions))
	for i, r := range audible.Regions {
		choices[i] = r.Name
	}
	ri := prompt.Radio(
		"Pick region",
		choices,
	)
	if ri < 0 {
		ri = 0
	}
	return audible.Regions[ri]
}

func DownloadLibrary(ctx context.Context, c *audible.Client) error {
	activationBytes, err := c.GetActivationBytes(ctx)
	if err != nil {
		return fmt.Errorf("Error getting activation bytes: %s", err)
	}
	fmt.Printf("Activation Bytes: %s\n", string(activationBytes))

	books, err := c.GetLibrary(ctx)
	if err != nil {
		return fmt.Errorf("Error reading library: %s\n", err)
	}

	// write info.txt file for all books already downloaded
	for _, b := range books {
		dir := b.Dir()
		fi, err := os.Lstat(dir)
		if err == nil && fi.IsDir() {
			// book exists
			if err := WriteInfoFile(b); err != nil {
				fmt.Printf("Error writing info file for %q: %s\n", b.Title, err)
			}
		}
	}

	books, err = GetNewBooks(books)
	if err != nil {
		return fmt.Errorf("Error filtering library: %s\n", err)
	}

	if len(books) == 0 {
		fmt.Printf("You have downloaded all the books from your Audible library.\n")
		return nil
	}

outer:
	for {
		answer := strings.ToLower(prompt.String(fmt.Sprintf("Download %d new books from your Audible library? (yes/no/list)", len(books)), prompt.Required))
		switch answer {
		case "yes":
			break outer
		case "list":
			for i, b := range books {
				fmt.Printf("%02d) %s by %s\n", i+1, b.Title, strings.Join(b.Authors, ", "))
			}
			break
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
		return fmt.Errorf("Error initializing downloader: %s\n", err)
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

		dir := book.Dir()
		os.MkdirAll(dir, 0755)

		if err := WriteInfoFile(book); err != nil {
			pushError(fmt.Errorf("Error writing info file for %q: %s", book.Title, err))
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
					downloader.DownloadOptionFilter(func(dl *downloader.Download) bool {
						if strings.Contains(dl.OutputPath(), "Part") {
							return false
						}
						return true
					}),
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
					pushError(fmt.Errorf("Error downloading %s: %s\n", u, err))
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
						pushError(fmt.Errorf("Error decrypting %s: %s", u, err))
					}

					// remove original .aax file
					if err := os.Remove(outPath); err != nil {
						pushError(fmt.Errorf("Error removing .aax file for %q: %s", book.Title, err))
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

func WriteInfoFile(book *audible.Book) error {
	f, err := os.OpenFile(filepath.Join(book.Dir(), "info.txt"), os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	return book.WriteInfo(f)
}

func GetNewBooks(books []*audible.Book) ([]*audible.Book, error) {
	newBooks := make([]*audible.Book, 0, len(books))
	for _, b := range books {
		dir := b.Dir()
		fi, err := os.Lstat(dir)
		if err == nil && fi.IsDir() {
			// book's dir exists, check if it has an mp4
			m, err := filepath.Glob(filepath.Join(dir, "*.mp4"))
			if err != nil {
				return nil, err
			}
			if len(m) > 0 {
				// there's at least one mp4 in the book's dir
				// assume the book has been downloaded
				continue
			}
		}

		// the book is assumed to need downloading
		newBooks = append(newBooks, b)
	}
	return newBooks, nil
}
