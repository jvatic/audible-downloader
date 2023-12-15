package library

import (
	"fmt"
	"image/color"
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
	"github.com/jvatic/audible-downloader/internal/downloader"
	"github.com/jvatic/audible-downloader/internal/errgroup"
	"github.com/jvatic/audible-downloader/internal/ffmpeg"
	"github.com/jvatic/audible-downloader/internal/progress"
	"github.com/jvatic/audible-downloader/internal/utils"
	log "github.com/sirupsen/logrus"
)

type Action = func(s *State)

type State struct {
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

func NewState(client *audible.Client, activationBytes []byte, books []*audible.Book) chan<- Action {
	state := &State{
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

	actionQueue := make(chan Action)

	go func() {
		for {
			fn, ok := <-actionQueue
			if !ok {
				return
			}
			fn(state)
		}
	}()

	return actionQueue
}

func (s *State) SetNumSelected(num int) {
	s.numSelected = num
	s.handleNumSelectedChange(num)
}

func (s *State) handleNumSelectedChange(num int) {
	s.downloadBtnCh <- components.ButtonActionSetText(s.GetDownloadBtnText())

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

func (s *State) SetSelectedDirPath(v string) {
	s.selectedDirPathMtx.Lock()
	defer s.selectedDirPathMtx.Unlock()
	s.selectedDirPath = v
}

func (s *State) SetBookLocalPath(index int, localPath string) {
	s.booksMtx.Lock()
	defer s.booksMtx.Unlock()
	s.books[index].LocalPath = localPath
}

func (s *State) SetBookIndexForID(id string, index int) {
	s.bookIndicesByID[id] = index
}

func (s *State) SetBookDownloaded(index int, downloaded bool) {
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

func SetBookDownloaded(index int, downloaded bool) Action {
	return func(s *State) {
		s.SetBookDownloaded(index, downloaded)
	}
}

func SetBookCheckboxCh(index int, ch chan<- components.CheckboxAction) Action {
	return func(s *State) {
		s.bookCheckboxChs[index] = ch
	}
}

func SetBookStatusText(index int, text string) Action {
	return func(s *State) {
		s.bookStatusChs[index] <- text
	}
}

func SetSelectedDir(uri fyne.ListableURI) Action {
	return func(s *State) {
		s.selectedDirURI = uri
		s.selectedDirPath = PathFromFyneURI(uri)
		defer func(text string) { s.dirPickerBtnCh <- components.ButtonActionSetText(text) }(s.GetDirPickerBtnText())

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
			s.SetBookDownloaded(i, false)
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
			log.Debugf("Found %d local books", len(localBooks))
			n := s.numSelected
			for _, b := range localBooks {
				if bi, ok := s.GetBookIndexForID(b.ID()); ok {
					s.books[bi].LocalPath = b.LocalPath
					ext := filepath.Ext(b.LocalPath)
					if ext != ".mp4" {
						// we're assuming any .mp4 found is downloaded
						continue
					}
					s.SetBookDownloaded(bi, true)
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
				} else {
					log.Debugf("unable to match local book: %s (%s)", b.ID(), b.Title)
				}
			}
			s.SetNumSelected(n)
		}
	}
}

func SetDownloading(isDownloading bool) Action {
	return func(s *State) {
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

func (s *State) GetBooks() []*audible.Book {
	s.booksMtx.RLock()
	defer s.booksMtx.RUnlock()
	return s.books
}

func (s *State) GetBooksLen() int {
	s.booksMtx.RLock()
	defer s.booksMtx.RUnlock()
	return len(s.books)
}

func (s *State) GetBook(index int) *audible.Book {
	s.booksMtx.RLock()
	defer s.booksMtx.RUnlock()
	if index >= len(s.books) {
		return nil
	}
	return s.books[index]
}

func (s *State) GetBookIndicesByID() map[string]int {
	s.bookIndicesByIDMtx.RLock()
	defer s.bookIndicesByIDMtx.RUnlock()
	return s.bookIndicesByID
}

func (s *State) GetBookIndexForID(id string) (int, bool) {
	index, ok := s.bookIndicesByID[id]
	return index, ok
}

func (s *State) GetDownloadedBooks() []*audible.Book {
	s.downloadedBookIndicesMtx.RLock()
	defer s.downloadedBookIndicesMtx.RUnlock()
	books := make([]*audible.Book, 0, len(s.downloadedBookIndices))
	for i := range s.downloadedBookIndices {
		books = append(books, s.GetBook(i))
	}
	sort.Sort(audible.ByTitle(books))
	return books
}

func (s *State) GetDirPickerBtnText() string {
	if s.selectedDirURI != nil {
		return FormatFilePath(s.selectedDirPath, 300)
	}
	return "Select output folder"
}

func (s *State) IsBookDownloaded(index int) bool {
	s.downloadedBookIndicesMtx.Lock()
	defer s.downloadedBookIndicesMtx.Unlock()
	_, ok := s.downloadedBookIndices[index]
	return ok
}

func (s *State) GetDownloadBtnText() string {
	return fmt.Sprintf("Download Selected (%d)", s.numSelected)
}

func GetActivationBytes(actionQueue chan<- Action) []byte {
	val := make(chan []byte)
	defer close(val)
	actionQueue <- func(s *State) {
		val <- s.activationBytes
	}
	return <-val
}

func GetSelectedDirPath(actionQueue chan<- Action) string {
	val := make(chan string)
	defer close(val)
	actionQueue <- func(s *State) {
		val <- s.selectedDirPath
	}
	return <-val
}

func GetDstPath(actionQueue chan<- Action, b *audible.Book) string {
	val := make(chan string)
	defer close(val)
	actionQueue <- func(s *State) {
		if s.getDstPath == nil {
			val <- ""
		} else {
			val <- s.getDstPath(b)
		}
	}
	return <-val
}

func GetDirPickerBtnText(actionQueue chan<- Action) string {
	textCh := make(chan string)
	defer close(textCh)
	actionQueue <- func(s *State) {
		textCh <- s.GetDirPickerBtnText()
	}
	return <-textCh
}

func GetDownloadBtnText(actionQueue chan<- Action) string {
	valCh := make(chan string)
	actionQueue <- func(s *State) {
		valCh <- s.GetDownloadBtnText()
	}
	val := <-valCh
	close(valCh)
	return val
}

func GetCookieJar(actionQueue chan<- Action) http.CookieJar {
	jarCh := make(chan http.CookieJar)
	defer close(jarCh)
	actionQueue <- func(s *State) {
		jarCh <- s.Client.Jar
	}
	return <-jarCh
}

func IsBookSelected(actionQueue chan<- Action, index int) bool {
	valCh := make(chan bool)
	actionQueue <- func(s *State) {
		valCh <- components.IsCheckboxChecked(s.bookCheckboxChs[index])
	}
	selected := <-valCh
	close(valCh)
	return selected
}

func IsBookDownloaded(actionQueue chan<- Action, index int) bool {
	valCh := make(chan bool)
	actionQueue <- func(s *State) {
		valCh <- s.IsBookDownloaded(index)
	}
	downloaded := <-valCh
	close(valCh)
	return downloaded
}

func BookCheckboxAction(index int, action components.CheckboxAction) Action {
	return func(s *State) {
		s.bookCheckboxChs[index] <- action
	}
}

func BookProgressBarAction(index int, action components.ProgressBarAction) Action {
	return func(s *State) {
		pbCh := s.bookProgressBarChs[index]
		pbCh <- action
	}
}

func BookProgressBarMaybeShow(index int) Action {
	return func(s *State) {
		cCh := s.bookCheckboxChs[index]
		pbCh := s.bookProgressBarChs[index]
		if components.IsProgressBarHidden(pbCh) && components.IsCheckboxChecked(cCh) {
			pbCh <- components.ProgressBarActionShow()
		}
	}
}

func MainProgressBarAction(action components.ProgressBarAction) Action {
	return func(s *State) {
		s.progressBarCh <- action
	}
}

func StartDownloads(actionQueue chan<- Action) error {
	actionQueue <- SetDownloading(true)
	defer func() { actionQueue <- SetDownloading(false) }()

	dlm, err := downloader.NewDownloader()
	if err != nil {
		return fmt.Errorf("Error initializing downloader: %s\n", err)
	}

	eg := errgroup.NewErrGroup()

	var books []*audible.Book
	actionQueue <- func(s *State) {
		books = s.books
	}

	pg := progress.NewComposite()
	bpg := make([]progress.ProgressComposite, len(books))
	for i := range bpg {
		bpg[i] = progress.NewComposite()
		pg.Add(bpg[i])
	}

	client := &http.Client{Jar: GetCookieJar(actionQueue)}

	downloadBook := func(index int, book *audible.Book, bookProgress progress.ProgressComposite) error {
		dstPath := filepath.Join(GetSelectedDirPath(actionQueue), GetDstPath(actionQueue, book))
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
						downloader.DownloadOptionProgress(func(totalBytes int64, completedBytes int64) {
							actionQueue <- SetBookStatusText(index, "Downloading...")
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
							ffmpeg.ActivationBytes(string(GetActivationBytes(actionQueue))),
							ffmpeg.Progress(func(totalBytes int64, completedBytes int64) {
								actionQueue <- SetBookStatusText(index, "Decrypting...")
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
		if IsBookSelected(actionQueue, i) {
			func(i int, book *audible.Book, pg progress.ProgressComposite) {
				eg.Add(func() error {
					actionQueue <- SetBookStatusText(i, "Pending...")
					if err := downloadBook(i, book, pg); err != nil {
						actionQueue <- SetBookStatusText(i, "An error occured while downloading")
						return err
					}
					actionQueue <- BookCheckboxAction(i, components.CheckboxActionSetChecked(false))
					actionQueue <- BookCheckboxAction(i, components.CheckboxActionDisable())
					actionQueue <- SetBookDownloaded(i, true)
					actionQueue <- SetBookStatusText(i, "Downloaded")
					actionQueue <- BookProgressBarAction(i, components.ProgressBarActionHide())
					return nil
				})
			}(i, book, bpg[i])
		} else if IsBookDownloaded(actionQueue, i) {
			actionQueue <- BookCheckboxAction(i, components.CheckboxActionSetChecked(false))
			actionQueue <- BookCheckboxAction(i, components.CheckboxActionDisable())
			actionQueue <- SetBookStatusText(i, BookStatusText(book))
			actionQueue <- BookProgressBarAction(i, components.ProgressBarActionHide())
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
				actionQueue <- BookProgressBarAction(i, components.ProgressBarActionSetValue(p.GetPercent()))
				actionQueue <- BookProgressBarMaybeShow(i)
			}
			actionQueue <- MainProgressBarAction(components.ProgressBarActionSetValue(pg.GetPercent()))
		}
	}
}

func Run(w fyne.Window, renderQueue chan func(w fyne.Window), actionQueue chan<- Action) error {
	var mainUI fyne.CanvasObject
	var configUI fyne.CanvasObject
	done := make(chan struct{})

	dirPickerBtn, dirPickerBtnCh := components.NewButton(renderQueue,
		GetDirPickerBtnText(actionQueue),
		components.ButtonOptionOnTapped(func() {
			d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
				if err != nil {
					log.Errorf("Error selecting output dir: %#v\n", err)
					return
				}
				if uri == nil {
					return
				}
				actionQueue <- SetSelectedDir(uri)
			}, w)
			d.Show()
		}),
	)
	actionQueue <- func(s *State) {
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
				actionQueue <- SetSelectedDir(uri)
			}, w)
			d.Show()
		}),
	)
	actionQueue <- func(s *State) {
		s.dirEntryBtnCh = dirEntryBtnCh
	}

	dirCreateBtn, dirCreateBtnCh := components.NewButton(renderQueue, "",
		components.ButtonOptionIcon(theme.FolderNewIcon()),
		components.ButtonOptionOnTapped(func() {
			d := dialog.NewEntryDialog("Create folder", fmt.Sprintf("%s%s", FormatFilePath(GetSelectedDirPath(actionQueue), 200), string(filepath.Separator)), func(str string) {
				path := filepath.Join(GetSelectedDirPath(actionQueue), str)
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
				actionQueue <- SetSelectedDir(uri)
			}, w)
			d.Show()
		}),
	)
	dirCreateBtnCh <- components.ButtonActionDisable()
	actionQueue <- func(s *State) {
		s.dirCreateBtnCh = dirCreateBtnCh
	}

	configUI = buildConfigUI(renderQueue, actionQueue, func() {
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
	actionQueue <- func(s *State) {
		s.configBtnCh = configBtnCh
	}

	downloadBtn, downloadBtnCh := components.NewButton(renderQueue,
		GetDownloadBtnText(actionQueue),
		components.ButtonOptionIcon(theme.DownloadIcon()),
		components.ButtonOptionOnTapped(func() {
			go StartDownloads(actionQueue)
		}),
	)
	downloadBtnCh <- components.ButtonActionDisable()
	actionQueue <- func(s *State) {
		s.downloadBtnCh = downloadBtnCh
	}

	progressBar, progressBarCh := components.NewProgressBar(renderQueue)
	progressBarCh <- components.ProgressBarActionHide()
	actionQueue <- func(s *State) {
		s.progressBarCh = progressBarCh
	}

	controlCheckbox, controlCheckboxCh := components.NewCheckbox(renderQueue, "",
		components.CheckboxOptionOnChange(func(checked bool) {
			actionQueue <- func(s *State) {
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
	actionQueue <- func(s *State) {
		s.controlCheckboxCh = controlCheckboxCh
	}

	booksCh := make(chan []*audible.Book)
	actionQueue <- func(s *State) {
		booksCh <- s.books
	}
	books := <-booksCh
	close(booksCh)

	bookCheckboxes := make([]*widget.Check, 0, len(books))
	for i := 0; i < len(books); i++ {
		checkbox, checkboxCh := components.NewCheckbox(renderQueue, "",
			components.CheckboxOptionOnChange(func(checked bool) {
				if checked {
					actionQueue <- func(s *State) {
						s.SetNumSelected(s.numSelected + 1)
					}
				} else {
					actionQueue <- func(s *State) {
						s.SetNumSelected(s.numSelected - 1)
					}
				}
			}),
		)
		checkboxCh <- components.CheckboxActionSetChecked(true)
		actionQueue <- SetBookCheckboxCh(i, checkboxCh)
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
				actionQueue <- func(s *State) {
					s.bookStatusChs[i] = statusTextCh
				}

				pb, pbCh := components.NewProgressBar(renderQueue)
				pbCh <- components.ProgressBarActionHide()
				actionQueue <- func(s *State) {
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
