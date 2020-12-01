package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jvatic/audible-downloader/audible"
	"github.com/jvatic/audible-downloader/audible/auth"
	"github.com/jvatic/audible-downloader/internal/downloader"
	"github.com/jvatic/audible-downloader/internal/ffmpeg"
	"github.com/jvatic/audible-downloader/internal/utils"
	mpb "github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	region := RegionPrompt()
	u, err := url.Parse(fmt.Sprintf("https://www.audible.%s", region.TLD))
	if err != nil {
		log.Fatalf("Unable to parse domain: %v", err)
	}
	fmt.Printf("Using %s (%s)\n", u.Host, region.Name)

	username := Prompt("Audible Username", true)
	password := SecurePrompt("Audible Password", true)
	c, err := auth.NewClient(auth.OptionBaseURL(u.String()), auth.OptionUsername(username), auth.OptionPassword(password), auth.OptionCaptcha(func(imgURL string) string {
		return Prompt(fmt.Sprintf("%s\nPlease enter captcha from above URL", imgURL), true)
	}), auth.OptionAuthCode(func() string {
		return Prompt("Auth Code", true)
	}))
	if err != nil {
		log.Fatalf("Error creating client: %s\n", err)
	}

	if err := c.Authenticate(); err != nil {
		log.Fatalf("Error authenticating: %s\n", err)
	}

	if err := DownloadLibrary(c); err != nil {
		log.Fatalf("Error downloading library: %v", err)
	}
}

func RegionPrompt() audible.Region {
	for i, l := range audible.Regions {
		fmt.Printf("%d) %s\n", i+1, l.Name)
	}
	r := audible.Regions[0]
	for {
		if str := Prompt(fmt.Sprintf("Pick region (1-%d, default: 1)", len(audible.Regions)), false); str != "" {
			n, err := strconv.Atoi(str)
			if err != nil || n > len(audible.Regions) || n < 1 {
				fmt.Printf("Invalid choice: %q, please enter a number for the options listed above.\n", str)
				continue
			}
			r = audible.Regions[n-1]
			break
		} else {
			// no option given, using default
			break
		}
	}
	return r
}

func Prompt(msg string, required bool) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s: ", msg)
		if line, err := reader.ReadString('\n'); err == nil {
			line = strings.TrimSpace(line)
			if !required || line != "" {
				return line
			}
		} else {
			break
		}
	}
	return ""
}

func SecurePrompt(msg string, required bool) string {
	defer fmt.Println("")
	for {
		fmt.Printf("%s: ", msg)
		if lineBytes, err := terminal.ReadPassword(int(syscall.Stdin)); err == nil {
			line := strings.TrimSpace(string(lineBytes))
			if !required || line != "" {
				return line
			}
		} else {
			break
		}
	}
	return ""
}

func DownloadLibrary(c *auth.Client) error {
	activationBytes, err := c.GetActivationBytes()
	if err != nil {
		return fmt.Errorf("Error getting activation bytes: %s", err)
	}
	fmt.Printf("Activation Bytes: %s\n", string(activationBytes))

	books, err := audible.GetLibrary(c)
	if err != nil {
		return fmt.Errorf("Error reading library: %s\n", err)
	}

	books, err = GetNewBooks(books)
	if err != nil {
		return fmt.Errorf("Error filtering library: %s\n", err)
	}

	if len(books) == 0 {
		fmt.Printf("You have downloaded all the books from your Audible library.\n")
		return nil
	}

	if strings.ToLower(Prompt(fmt.Sprintf("Download %d new books from your Audible library? (yes/no)", len(books)), true)) != "yes" {
		return fmt.Errorf("download aborted")
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
