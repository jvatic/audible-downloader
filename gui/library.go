package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/storage"
	"fyne.io/fyne/theme"
	"fyne.io/fyne/widget"
	"github.com/jvatic/audible-downloader/audible"
	"github.com/jvatic/audible-downloader/gui/components"
	"github.com/jvatic/audible-downloader/internal/common"
	"github.com/jvatic/audible-downloader/internal/config"
	"github.com/jvatic/audible-downloader/internal/downloader"
	"github.com/jvatic/audible-downloader/internal/errgroup"
	"github.com/jvatic/audible-downloader/internal/ffmpeg"
	"github.com/jvatic/audible-downloader/internal/progress"
	"github.com/jvatic/audible-downloader/internal/utils"
	log "github.com/sirupsen/logrus"
)

type libState struct {
	Client *audible.Client

	// Data
	activationBytes          []byte
	activationBytesMtx       sync.RWMutex
	numSelected              int
	numSelectedMtx           sync.RWMutex
	selectedDirURI           fyne.ListableURI
	selectedDirURIMtx        sync.RWMutex
	selectedDirPath          string
	selectedDirPathMtx       sync.RWMutex
	books                    []*audible.Book
	booksMtx                 sync.RWMutex
	bookIndicesByID          map[string]int
	bookIndicesByIDMtx       sync.RWMutex
	downloadedBookIndices    map[int]struct{}
	downloadedBookIndicesMtx sync.RWMutex
	downloadedBooks          []*audible.Book
	downloadedBooksMtx       sync.RWMutex
	getDstPath               func(b *audible.Book) string
	getDstPathMtx            sync.RWMutex

	// UI action channels
	dirPickerBtnCh     chan<- components.ButtonAction
	dirEntryBtnCh      chan<- components.ButtonAction
	dirCreateBtnCh     chan<- components.ButtonAction
	configBtnCh        chan<- components.ButtonAction
	downloadBtnCh      chan<- components.ButtonAction
	progressBarCh      chan<- components.ProgressBarAction
	controlCheckboxCh  chan<- components.CheckboxAction
	bookCheckboxChs    []chan<- components.CheckboxAction
	bookStatusChs      []chan<- string
	bookProgressBarChs []chan<- components.ProgressBarAction
}

// Data

func GetActivationBytes(stateCh chan<- LibStateAction) []byte {
	activationBytesCh := make(chan []byte)
	defer close(activationBytesCh)
	stateCh <- func(s *libState) {
		activationBytesCh <- s.activationBytes
	}
	return <-activationBytesCh
}

func (s *libState) SetNumSelected(num int) {
	s.numSelected = num
	s.handleNumSelectedChange(num)
}

func (s *libState) handleNumSelectedChange(num int) {
	s.downloadBtnCh <- components.ButtonActionSetText(s.DownloadBtnText())

	if num > 0 && s.selectedDirURI != nil {
		s.downloadBtnCh <- components.ButtonActionEnable()
	} else {
		s.downloadBtnCh <- components.ButtonActionDisable()
	}

	if num == len(s.books)-len(s.downloadedBooks) && num > 0 {
		s.controlCheckboxCh <- components.CheckboxActionSetChecked(true)
	} else {
		s.controlCheckboxCh <- components.CheckboxActionSetChecked(false)
	}
}

func GetSelectedDirPath(stateCh chan<- LibStateAction) string {
	dirPathCh := make(chan string)
	defer close(dirPathCh)
	stateCh <- func(s *libState) {
		dirPathCh <- s.selectedDirPath
	}
	return <-dirPathCh
}

func (s *libState) SetSelectedDirPath(v string) {
	s.selectedDirPathMtx.Lock()
	defer s.selectedDirPathMtx.Unlock()
	s.selectedDirPath = v
}

func (s *libState) GetBooks() []*audible.Book {
	s.booksMtx.RLock()
	defer s.booksMtx.RUnlock()
	return s.books
}

func (s *libState) GetBooksLen() int {
	s.booksMtx.RLock()
	defer s.booksMtx.RUnlock()
	return len(s.books)
}

func (s *libState) GetBook(index int) *audible.Book {
	s.booksMtx.RLock()
	defer s.booksMtx.RUnlock()
	if index >= len(s.books) {
		return nil
	}
	return s.books[index]
}

func (s *libState) SetBookLocalPath(index int, localPath string) {
	s.booksMtx.Lock()
	defer s.booksMtx.Unlock()
	s.books[index].LocalPath = localPath
}

func (s *libState) GetBookIndicesByID() map[string]int {
	s.bookIndicesByIDMtx.RLock()
	defer s.bookIndicesByIDMtx.RUnlock()
	return s.bookIndicesByID
}

func (s *libState) GetBookIndexForID(id string) (int, bool) {
	index, ok := s.bookIndicesByID[id]
	return index, ok
}

func (s *libState) SetBookIndexForID(id string, index int) {
	s.bookIndicesByID[id] = index
}

func LSAMarkBookDownloaded(index int, downloaded bool) LibStateAction {
	return func(s *libState) {
		s.MarkBookDownloaded(index, downloaded)
	}
}

func LSASetBookCheckboxCh(index int, ch chan<- components.CheckboxAction) LibStateAction {
	return func(s *libState) {
		s.bookCheckboxChs[index] = ch
	}
}

func LSABookCheckboxAction(index int, action components.CheckboxAction) LibStateAction {
	return func(s *libState) {
		s.bookCheckboxChs[index] <- action
	}
}

func (s *libState) MarkBookDownloaded(index int, downloaded bool) {
	if downloaded {
		b := s.books[index]
		if b == nil {
			return
		}
		s.downloadedBookIndices[index] = struct{}{}
	} else {
		delete(s.downloadedBookIndices, index)
	}
}

func (s *libState) IsBookDownloaded(index int) bool {
	s.downloadedBookIndicesMtx.Lock()
	defer s.downloadedBookIndicesMtx.Unlock()
	_, ok := s.downloadedBookIndices[index]
	return ok
}

func (s *libState) GetDownloadedBooks() []*audible.Book {
	s.downloadedBookIndicesMtx.RLock()
	defer s.downloadedBookIndicesMtx.RUnlock()
	books := make([]*audible.Book, 0, len(s.downloadedBookIndices))
	for i := range s.downloadedBookIndices {
		books = append(books, s.GetBook(i))
	}
	sort.Sort(audible.ByTitle(books))
	return books
}

func GetDstPath(stateCh chan<- LibStateAction, b *audible.Book) string {
	dstPathCh := make(chan string)
	defer close(dstPathCh)
	stateCh <- func(s *libState) {
		if s.getDstPath == nil {
			dstPathCh <- ""
		} else {
			dstPathCh <- s.getDstPath(b)
		}
	}
	return <-dstPathCh
}

// UI

type LibStateAction = func(s *libState)

func LSABookProgressBarAction(index int, action components.ProgressBarAction) LibStateAction {
	return func(s *libState) {
		pbCh := s.bookProgressBarChs[index]
		pbCh <- action
	}
}

func LSAProgressBarAction(action components.ProgressBarAction) LibStateAction {
	return func(s *libState) {
		s.progressBarCh <- action
	}
}

func LSABookProgressBarMaybeShow(index int) LibStateAction {
	return func(s *libState) {
		cCh := s.bookCheckboxChs[index]
		pbCh := s.bookProgressBarChs[index]
		if components.IsProgressBarHidden(pbCh) && components.IsCheckboxChecked(cCh) {
			pbCh <- components.ProgressBarActionShow()
		}
	}
}

func LSASetBookStatusText(index int, text string) LibStateAction {
	return func(s *libState) {
		s.bookStatusChs[index] <- text
	}
}

func IsBookSelected(stateCh chan<- LibStateAction, index int) bool {
	valCh := make(chan bool)
	stateCh <- func(s *libState) {
		valCh <- components.IsCheckboxChecked(s.bookCheckboxChs[index])
	}
	selected := <-valCh
	close(valCh)
	return selected
}

func IsBookDownloaded(stateCh chan<- LibStateAction, index int) bool {
	valCh := make(chan bool)
	stateCh <- func(s *libState) {
		valCh <- s.IsBookDownloaded(index)
	}
	downloaded := <-valCh
	close(valCh)
	return downloaded
}

func LSASetSelectedDir(uri fyne.ListableURI) LibStateAction {
	return func(s *libState) {
		s.selectedDirURI = uri
		s.selectedDirPath = PathFromFyneURI(uri)
		defer func(text string) { s.dirPickerBtnCh <- components.ButtonActionSetText(text) }(s.DirPickerBtnText())

		s.downloadedBookIndices = make(map[int]struct{})

		// we assume any disabled checkbox was disabled due to being downloaded, so
		// re-enable and select any disabled checkbox and mark it's book as not
		// downloaded unless/until we determine otherwise below
		n := s.numSelected
		for i := 0; i < len(s.books); i++ {
			ch := s.bookCheckboxChs[i]
			if ch != nil && components.IsCheckboxDisabled(ch) {
				ch <- components.CheckboxActionSetChecked(true)
				ch <- components.CheckboxActionEnable()
				n++
			}
			s.books[i].LocalPath = ""
			s.MarkBookDownloaded(i, false)
			s.bookStatusChs[i] <- BookStatusText(s.books[i])
		}
		s.SetNumSelected(n)

		if uri == nil {
			s.dirCreateBtnCh <- components.ButtonActionDisable()
		} else {
			s.dirCreateBtnCh <- components.ButtonActionEnable()

			// scan selected dir for audiobooks
			// and mark any matching books as downloaded and disable their checkbox
			localBooks, err := common.ListDownloadedBooks(s.selectedDirPath)
			if err != nil {
				log.Errorf("Error discovering downloaded books: %s", err)
			}
			n := s.numSelected
			for _, b := range localBooks {
				if bi, ok := s.GetBookIndexForID(b.ID()); ok {
					s.books[bi].LocalPath = b.LocalPath
					ext := filepath.Ext(b.LocalPath)
					if ext != ".mp4" {
						// we're assuming any .mp4 found is downloaded
						continue
					}
					s.MarkBookDownloaded(bi, true)
					if bi < len(s.bookStatusChs) {
						s.bookStatusChs[bi] <- BookStatusText(b)
					}
					if bi < len(s.bookCheckboxChs) {
						ch := s.bookCheckboxChs[bi]
						if components.IsCheckboxChecked(ch) {
							n--
						}
						ch <- components.CheckboxActionSetChecked(false)
						ch <- components.CheckboxActionDisable()
					}
				}
			}
			s.SetNumSelected(n)
		}
	}
}

func LSASetDownloading(isDownloading bool) LibStateAction {
	return func(s *libState) {
		if isDownloading {
			s.dirPickerBtnCh <- components.ButtonActionHide()
			s.dirEntryBtnCh <- components.ButtonActionHide()
			s.dirCreateBtnCh <- components.ButtonActionHide()
			s.downloadBtnCh <- components.ButtonActionHide()
			s.progressBarCh <- components.ProgressBarActionShow()
		} else {
			s.dirPickerBtnCh <- components.ButtonActionShow()
			s.dirEntryBtnCh <- components.ButtonActionShow()
			s.dirCreateBtnCh <- components.ButtonActionShow()
			s.downloadBtnCh <- components.ButtonActionShow()
			s.progressBarCh <- components.ProgressBarActionHide()
		}
	}
}

func NewLibState(client *audible.Client, activationBytes []byte, books []*audible.Book) chan<- LibStateAction {
	state := &libState{
		Client: client,

		// Data
		activationBytes:       activationBytes,
		numSelected:           len(books),
		books:                 books,
		bookIndicesByID:       make(map[string]int, len(books)),
		downloadedBookIndices: make(map[int]struct{}, len(books)),
		downloadedBooks:       make([]*audible.Book, 0, len(books)),
		getDstPath: common.CompilePathTemplate(
			common.DefaultPathTemplate,
			common.PathTemplateTitle(),
			common.PathTemplateShortTitle(),
			common.PathTemplateAuthor(1, ""),
		),

		// UI action chans
		bookCheckboxChs:    make([]chan<- components.CheckboxAction, len(books)),
		bookStatusChs:      make([]chan<- string, len(books)),
		bookProgressBarChs: make([]chan<- components.ProgressBarAction, len(books)),
	}

	// index books by author/title to make finding which ones are already
	// downloaded faster
	for i, b := range books {
		state.SetBookIndexForID(b.ID(), i)
	}

	stateCh := make(chan LibStateAction)

	go func() {
		for {
			fn, ok := <-stateCh
			if !ok {
				return
			}
			fn(state)
		}
	}()

	return stateCh
}

func (s *libState) DirPickerBtnText() string {
	if s.selectedDirURI != nil {
		return FormatFilePath(s.selectedDirPath, 300)
	}
	return "Select output folder"
}

func GetDirPickerBtnText(stateCh chan<- LibStateAction) string {
	textCh := make(chan string)
	defer close(textCh)
	stateCh <- func(s *libState) {
		textCh <- s.DirPickerBtnText()
	}
	return <-textCh
}

func (s *libState) DownloadBtnText() string {
	return fmt.Sprintf("Download Selected (%d)", s.numSelected)
}

func DownloadBtnText(stateCh chan<- LibStateAction) string {
	valCh := make(chan string)
	stateCh <- func(s *libState) {
		valCh <- s.DownloadBtnText()
	}
	val := <-valCh
	close(valCh)
	return val
}

func BookStatusText(b *audible.Book) string {
	if b.LocalPath == "" {
		return "Status: Not Downloaded"
	}
	return "Status: Downloaded"
}

func PathFromFyneURI(uri fyne.ListableURI) string {
	if uri == nil {
		return ""
	}
	return filepath.Join(strings.SplitAfter(strings.TrimPrefix(uri.String(), "file://"), "/")...)
}

func GetCookieJar(stateCh chan<- LibStateAction) http.CookieJar {
	jarCh := make(chan http.CookieJar)
	defer close(jarCh)
	stateCh <- func(s *libState) {
		jarCh <- s.Client.Jar
	}
	return <-jarCh
}

func StartDownloads(stateCh chan<- LibStateAction) error {
	stateCh <- LSASetDownloading(true)
	defer func() { stateCh <- LSASetDownloading(false) }()

	dlm, err := downloader.NewDownloader()
	if err != nil {
		return fmt.Errorf("Error initializing downloader: %s\n", err)
	}

	eg := errgroup.NewErrGroup()

	var books []*audible.Book
	stateCh <- func(s *libState) {
		books = s.books
	}

	pg := progress.NewComposite()
	bpg := make([]progress.ProgressComposite, len(books))
	for i := range bpg {
		bpg[i] = progress.NewComposite()
		pg.Add(bpg[i])
	}

	client := &http.Client{Jar: GetCookieJar(stateCh)}

	downloadBook := func(index int, book *audible.Book, bookProgress progress.ProgressComposite) error {
		dstPath := filepath.Join(GetSelectedDirPath(stateCh), GetDstPath(stateCh, book))
		dir := filepath.Dir(dstPath)
		os.MkdirAll(dir, 0755)

		if err := common.WriteInfoFile(dir, book); err != nil {
			log.Errorf("Error writing info file for %q: %s", book.Title, err)
		}

		bookeg := errgroup.NewErrGroup()
		for _, u := range book.DownloadURLs {
			func(u string) {
				bookeg.Add(func() error {
					dlProgress := progress.NewProgress()
					bookProgress.Add(dlProgress)

					// start the download
					var dl *downloader.Download
					dl = downloader.NewDownload(
						downloader.DownloadOptionDirname(dir),
						downloader.DownloadOptionURL(u),
						downloader.DownloadOptionDetectFilename(),
						downloader.DownloadOptionPreferFilename(utils.SwapFileExt(filepath.Base(dstPath), ".aax"), "audio/vnd.audible.aax"),
						downloader.DownloadOptionHTTPClient(client),
						downloader.DownloadOptionFilter(func(dl *downloader.Download) bool {
							if strings.Contains(dl.OutputPath(), "Part") {
								return false
							}
							return true
						}),
						downloader.DownloadOptionProgress(func(totalBytes int64, completedBytes int64) {
							stateCh <- LSASetBookStatusText(index, "Downloading...")
							dlProgress.SetTotal(totalBytes)
							dlProgress.SetCurrent(completedBytes)
						}),
					)
					dlm.Add(dl)

					// wait for it to complete
					if err := dl.Wait(); err != nil {
						if err == downloader.ErrAbort {
							log.Infof("Skipping %s\n", dl.OutputPath())
							return err
						}
						return fmt.Errorf("Error downloading %s: %w\n", u, err)
					}

					outPath := dl.OutputPath()

					// decrypt if it's an .aax file
					if filepath.Ext(outPath) == ".aax" {
						dcProgress := progress.NewProgress()
						bookProgress.Add(dcProgress)
						if err := ffmpeg.DecryptAudioBook(
							ffmpeg.InputPath(outPath),
							ffmpeg.OutputPath(utils.SwapFileExt(outPath, ".mp4")),
							ffmpeg.ActivationBytes(string(GetActivationBytes(stateCh))),
							ffmpeg.Progress(func(totalBytes int64, completedBytes int64) {
								stateCh <- LSASetBookStatusText(index, "Decrypting...")
								dcProgress.SetTotal(totalBytes)
								dcProgress.SetCurrent(completedBytes)
							}),
						); err != nil {
							return fmt.Errorf("Error decrypting %s: %w", u, err)
						}

						// remove original .aax file
						if err := os.Remove(outPath); err != nil {
							return fmt.Errorf("Error removing .aax file for %q: %w", book.Title, err)
						}
					}

					return nil
				})
			}(u)
		}
		if errs := bookeg.Wait(); len(errs) > 0 {
			for _, err := range errs {
				if err == downloader.ErrAbort {
					continue
				}
				log.Error(err)
			}
			return errs[0]
		}
		return nil
	}

	dlm.Start()
	for i, book := range books {
		if IsBookSelected(stateCh, i) {
			func(i int, book *audible.Book, pg progress.ProgressComposite) {
				eg.Add(func() error {
					stateCh <- LSASetBookStatusText(i, "Pending...")
					if err := downloadBook(i, book, pg); err != nil {
						stateCh <- LSASetBookStatusText(i, "An error occured while downloading")
						return err
					}
					stateCh <- LSABookCheckboxAction(i, components.CheckboxActionSetChecked(false))
					stateCh <- LSABookCheckboxAction(i, components.CheckboxActionDisable())
					stateCh <- LSAMarkBookDownloaded(i, true)
					stateCh <- LSASetBookStatusText(i, "Downloaded")
					stateCh <- LSABookProgressBarAction(i, components.ProgressBarActionHide())
					return nil
				})
			}(i, book, bpg[i])
		} else if IsBookDownloaded(stateCh, i) {
			stateCh <- LSABookCheckboxAction(i, components.CheckboxActionSetChecked(false))
			stateCh <- LSABookCheckboxAction(i, components.CheckboxActionDisable())
			stateCh <- LSASetBookStatusText(i, BookStatusText(book))
			stateCh <- LSABookProgressBarAction(i, components.ProgressBarActionHide())
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if errs := eg.Wait(); len(errs) > 0 {
			for _, err := range errs {
				log.Error(err)
			}
		}
		dlm.Wait()
	}()

	// update book progress bars until done
	for {
		select {
		case <-done:
			return nil
		case <-time.After(time.Second):
			for i, p := range bpg {
				stateCh <- LSABookProgressBarAction(i, components.ProgressBarActionSetValue(p.GetPercent()))
				stateCh <- LSABookProgressBarMaybeShow(i)
			}
			stateCh <- LSAProgressBarAction(components.ProgressBarActionSetValue(pg.GetPercent()))
		}
	}
}

func Library(w fyne.Window, renderQueue chan func(w fyne.Window), stateCh chan<- LibStateAction) error {
	var mainUI fyne.CanvasObject
	var configUI fyne.CanvasObject
	done := make(chan struct{})

	dirPickerBtn, dirPickerBtnCh := components.NewButton(renderQueue,
		GetDirPickerBtnText(stateCh),
		components.ButtonOptionOnTapped(func() {
			d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
				if err != nil {
					log.Errorf("Error selecting output dir: %#v\n", err)
					return
				}
				if uri == nil {
					return
				}
				stateCh <- LSASetSelectedDir(uri)
			}, w)
			d.Show()
		}),
	)
	stateCh <- func(s *libState) {
		s.dirPickerBtnCh = dirPickerBtnCh
	}

	dirEntryBtn, dirEntryBtnCh := components.NewButton(renderQueue, "",
		components.ButtonOptionIcon(theme.FolderIcon()),
		components.ButtonOptionOnTapped(func() {
			d := dialog.NewEntryDialog("Please enter the full path for the desired output folder", "", func(str string) {
				if str == "" {
					return
				}
				uri, err := storage.ListerForURI(storage.NewFileURI(str))
				if err != nil {
					log.Error(err)
					dialog.ShowError(err, w)
					return
				}
				stateCh <- LSASetSelectedDir(uri)
			}, w)
			d.Show()
		}),
	)
	stateCh <- func(s *libState) {
		s.dirEntryBtnCh = dirEntryBtnCh
	}

	dirCreateBtn, dirCreateBtnCh := components.NewButton(renderQueue, "",
		components.ButtonOptionIcon(theme.FolderNewIcon()),
		components.ButtonOptionOnTapped(func() {
			d := dialog.NewEntryDialog("Create folder", fmt.Sprintf("%s%s", FormatFilePath(GetSelectedDirPath(stateCh), 200), string(filepath.Separator)), func(str string) {
				path := filepath.Join(GetSelectedDirPath(stateCh), str)
				if err := os.Mkdir(path, 0755); err != nil {
					log.Error(err)
					dialog.ShowError(err, w)
					return
				}
				uri, err := storage.ListerForURI(storage.NewFileURI(path))
				if err != nil {
					log.Error(err)
					dialog.ShowError(err, w)
					return
				}
				stateCh <- LSASetSelectedDir(uri)
			}, w)
			d.Show()
		}),
	)
	dirCreateBtnCh <- components.ButtonActionDisable()
	stateCh <- func(s *libState) {
		s.dirCreateBtnCh = dirCreateBtnCh
	}

	configUI = BuildConfigUI(renderQueue, stateCh, func() {
		// called when config UI closed
		renderQueue <- func(w fyne.Window) {
			w.SetContent(mainUI)
		}
	})
	configBtn, configBtnCh := components.NewButton(renderQueue, "",
		components.ButtonOptionIcon(theme.SettingsIcon()),
		components.ButtonOptionOnTapped(func() {
			renderQueue <- func(w fyne.Window) {
				w.SetContent(configUI)
			}
		}),
	)
	stateCh <- func(s *libState) {
		s.configBtnCh = configBtnCh
	}

	downloadBtn, downloadBtnCh := components.NewButton(renderQueue,
		DownloadBtnText(stateCh),
		components.ButtonOptionIcon(theme.DownloadIcon()),
		components.ButtonOptionOnTapped(func() {
			go StartDownloads(stateCh)
		}),
	)
	downloadBtnCh <- components.ButtonActionDisable()
	stateCh <- func(s *libState) {
		s.downloadBtnCh = downloadBtnCh
	}

	progressBar, progressBarCh := components.NewProgressBar(renderQueue)
	progressBarCh <- components.ProgressBarActionHide()
	stateCh <- func(s *libState) {
		s.progressBarCh = progressBarCh
	}

	controlCheckbox, controlCheckboxCh := components.NewCheckbox(renderQueue, "",
		components.CheckboxOptionOnChange(func(checked bool) {
			stateCh <- func(s *libState) {
				n := 0
				for _, ch := range s.bookCheckboxChs {
					if components.IsCheckboxDisabled(ch) {
						continue
					}
					n++
					ch <- components.CheckboxActionSetChecked(checked)
				}
				if checked {
					s.SetNumSelected(n)
				} else {
					s.SetNumSelected(0)
				}
			}
		}),
	)
	controlCheckboxCh <- components.CheckboxActionSetChecked(true)
	stateCh <- func(s *libState) {
		s.controlCheckboxCh = controlCheckboxCh
	}

	booksCh := make(chan []*audible.Book)
	stateCh <- func(s *libState) {
		booksCh <- s.books
	}
	books := <-booksCh
	close(booksCh)

	bookCheckboxes := make([]*widget.Check, 0, len(books))
	for i := 0; i < len(books); i++ {
		checkbox, checkboxCh := components.NewCheckbox(renderQueue, "",
			components.CheckboxOptionOnChange(func(checked bool) {
				if checked {
					stateCh <- func(s *libState) {
						s.SetNumSelected(s.numSelected + 1)
					}
				} else {
					stateCh <- func(s *libState) {
						s.SetNumSelected(s.numSelected - 1)
					}
				}
			}),
		)
		checkboxCh <- components.CheckboxActionSetChecked(true)
		stateCh <- LSASetBookCheckboxCh(i, checkboxCh)
		bookCheckboxes = append(bookCheckboxes, checkbox)
	}

	mainUI = (func() fyne.CanvasObject {
		booksList := (func() fyne.CanvasObject {
			rows := make([]fyne.CanvasObject, 0, len(books))

			width := 700

			for i, b := range books {
				var thumbImg fyne.CanvasObject
				if b.ThumbImage == nil {
					thumbImg = canvas.NewText("N/A", color.Black)
				} else {
					img := canvas.NewImageFromImage(b.ThumbImage)
					img.FillMode = canvas.ImageFillContain
					thumbImg = img
				}

				statusText, statusTextCh := components.NewText(renderQueue, BookStatusText(b))
				stateCh <- func(s *libState) {
					s.bookStatusChs[i] = statusTextCh
				}

				pb, pbCh := components.NewProgressBar(renderQueue)
				pbCh <- components.ProgressBarActionHide()
				stateCh <- func(s *libState) {
					s.bookProgressBarChs[i] = pbCh
				}

				titleText, _ := components.NewWrappedText(renderQueue, b.Title, width, components.TextOptionBold())

				writtenByText, _ := components.NewWrappedText(
					renderQueue,
					fmt.Sprintf("Written by: %s", strings.Join(b.Authors, ", ")),
					width,
				)

				narratedByText, _ := components.NewWrappedText(
					renderQueue,
					fmt.Sprintf("Narrated by: %s", strings.Join(b.Narrators, ", ")),
					width,
				)

				rows = append(rows, container.NewHBox(
					bookCheckboxes[i],
					container.NewGridWrap(fyne.Size{Width: 150, Height: 150}, thumbImg),
					container.NewGridWrap(
						fyne.Size{Width: width},
						container.NewVBox(
							titleText,
							writtenByText,
							narratedByText,
							statusText,
							pb,
						),
					),
				))
			}

			return container.NewVBox(rows...)
		})()

		headerText, _ := components.NewText(renderQueue, "Library")
		header := container.NewCenter(
			headerText,
		)

		top := container.NewHBox(
			controlCheckbox,
			header,
			layout.NewSpacer(),
			configBtn,
		)

		bottom := container.NewHBox(
			dirPickerBtn,
			dirEntryBtn,
			dirCreateBtn,
			layout.NewSpacer(),
			downloadBtn,
			progressBar,
		)

		return components.ApplyTemplate(
			fyne.NewContainerWithLayout(
				layout.NewHBoxLayout(),
				layout.NewSpacer(),
				fyne.NewContainerWithLayout(
					layout.NewBorderLayout(top, bottom, nil, nil),
					top,
					container.NewVScroll(booksList),
					bottom,
				),
				layout.NewSpacer(),
			),
		)
	})()

	renderQueue <- func(w fyne.Window) {
		w.SetContent(mainUI)
	}
	<-done
	return nil
}

func BuildConfigUI(renderQueue chan<- func(w fyne.Window), stateCh chan<- LibStateAction, closeFunc func()) fyne.CanvasObject {
	var pathTemplateMtx sync.RWMutex
	pathTemplate := common.DefaultPathTemplate
	setPathTemplate := func(text string) {
		pathTemplateMtx.Lock()
		defer pathTemplateMtx.Unlock()
		pathTemplate = text
	}

	getPathTemplate := func() string {
		pathTemplateMtx.RLock()
		defer pathTemplateMtx.RUnlock()
		return pathTemplate
	}

	var maxAuthorsMtx sync.RWMutex
	maxAuthors := 1
	setMaxAuthors := func(num int) {
		maxAuthorsMtx.Lock()
		defer maxAuthorsMtx.Unlock()
		maxAuthors = num
	}

	getMaxAuthors := func() int {
		maxAuthorsMtx.RLock()
		defer maxAuthorsMtx.RUnlock()
		return maxAuthors
	}

	var authorSeparatorMtx sync.RWMutex
	authorSeparator := ", "
	setAuthorSeparator := func(sep string) {
		authorSeparatorMtx.Lock()
		defer authorSeparatorMtx.Unlock()
		authorSeparator = sep
	}

	getAuthorSeparator := func() string {
		authorSeparatorMtx.RLock()
		defer authorSeparatorMtx.RUnlock()
		return authorSeparator
	}

	updatePathTemplateSub := func() {
		stateCh <- func(s *libState) {
			authorSeparatorMtx.RLock()
			defer authorSeparatorMtx.RUnlock()
			maxAuthorsMtx.RLock()
			defer maxAuthorsMtx.RUnlock()
			s.getDstPath = common.CompilePathTemplate(
				getPathTemplate(),
				common.PathTemplateTitle(),
				common.PathTemplateShortTitle(),
				common.PathTemplateAuthor(maxAuthors, authorSeparator),
			)
		}
	}

	previewText, previewTextCh := components.NewText(renderQueue, "")
	updatePreviewText := func() {
		previewTextCh <- GetDstPath(stateCh, &common.SampleBook)
	}
	updatePreviewText()

	pathTemplateInput, pathTemplateInputInCh, pathTemplateInputOutCh := components.NewEntry(renderQueue)
	pathTemplateInputInCh <- getPathTemplate()
	go func() {
		for {
			text, ok := <-pathTemplateInputOutCh
			if !ok {
				return
			}
			setPathTemplate(text)
			updatePathTemplateSub()
			updatePreviewText()
		}
	}()

	maxAuthorsInput, maxAuthorsInputInCh, maxAuthorsInputOutCh := components.NewIntEntry(renderQueue)
	maxAuthorsInputInCh <- getMaxAuthors()
	go func() {
		for {
			num, ok := <-maxAuthorsInputOutCh
			if !ok {
				return
			}
			setMaxAuthors(num)
			updatePathTemplateSub()
			updatePreviewText()
		}
	}()

	authorSeparatorInput, authorSeparatorInputInCh, authorSeparatorInputOutCh := components.NewEntry(renderQueue)
	authorSeparatorInputInCh <- getAuthorSeparator()
	go func() {
		for {
			sep, ok := <-authorSeparatorInputOutCh
			if !ok {
				return
			}
			setAuthorSeparator(sep)
			updatePathTemplateSub()
			updatePreviewText()
		}
	}()

	return components.ApplyTemplate(
		container.NewVBox(
			container.NewCenter(
				components.NewImmutableText("Download Settings", components.TextOptionHeading(components.H1)),
			),
			container.NewVBox(
				components.NewImmutableText("Download Path Template", components.TextOptionBold()),
				pathTemplateInput,
				Indent(
					components.NewImmutableText("Format Options: ", components.TextOptionBold()),
					container.NewHBox(
						components.NewImmutableText("%TITLE%", components.TextOptionBold()),
						canvas.NewText(" - Full book title as seen in library", color.Black),
					),
					container.NewHBox(
						components.NewImmutableText("%SHORT_TITLE%", components.TextOptionBold()),
						canvas.NewText(" - Book title up to the first occurance of ", color.Black),
						components.NewImmutableText(":", components.TextOptionBold()),
					),
					components.NewImmutableText("%AUTHOR%", components.TextOptionBold()),
					Indent(
						container.NewHBox(
							components.NewImmutableText("Max number of authors to include (0 = unlimited): ", components.TextOptionBold()),
							maxAuthorsInput,
							components.NewImmutableText("Author separator: ", components.TextOptionBold()),
							authorSeparatorInput,
						),
					),
				),
				components.NewImmutableText("Preview: ", components.TextOptionBold()),
				previewText,
			),
			layout.NewSpacer(),
			container.NewHBox(
				layout.NewSpacer(),
				widget.NewButton("Cancel", closeFunc),
				widget.NewButton("Save", closeFunc),
			),
		),
	)
}

func SaveLibrary(books []*audible.Book) error {
	path := filepath.Join(config.Dir(), "books.json")
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(books); err != nil {
		return fmt.Errorf("error encoding library to json: %s", err)
	}
	var wg sync.WaitGroup
	for i, b := range books {
		wg.Add(1)
		go func(i int, img image.Image) {
			defer wg.Done()
			if err := SaveLibraryThumb(i, img); err != nil {
				log.Errorf("error saving thumb %02d: %s", i, err)
			}
		}(i, b.ThumbImage)
	}
	wg.Wait()
	return nil
}

func SaveLibraryThumb(i int, img image.Image) error {
	path := filepath.Join(config.Dir(), fmt.Sprintf("%02d.jpeg", i))
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer file.Close()
	return jpeg.Encode(file, img, &jpeg.Options{Quality: 100})
}

func LoadLibrary() ([]*audible.Book, error) {
	path := filepath.Join(config.Dir(), "books.json")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var books []*audible.Book
	if err := json.NewDecoder(file).Decode(&books); err != nil {
		return nil, fmt.Errorf("error decoding library from json: %s", err)
	}
	var wg sync.WaitGroup
	for i, b := range books {
		wg.Add(1)
		go func(i int, b *audible.Book) {
			defer wg.Done()
			img, err := LoadLibraryThumb(i)
			if err != nil {
				log.Errorf("error decoding library thumb %02d: %s", i, err)
				return
			}
			b.ThumbImage = img
		}(i, b)
	}
	wg.Wait()
	return books, nil
}

func LoadLibraryThumb(i int) (image.Image, error) {
	path := filepath.Join(config.Dir(), fmt.Sprintf("%02d.jpeg", i))
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return jpeg.Decode(file)
}
