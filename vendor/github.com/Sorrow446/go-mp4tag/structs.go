package mp4tag

type Tags struct {
	Album       string
	AlbumArtist string
	Artist      string
	Comment     string
	Composer    string
	Copyright   string
	Cover       []byte
	Custom      map[string]string
	Delete      []string
	DiskNumber  int
	DiskTotal   int
	Genre       string
	Label       string
	Title       string
	TrackNumber int
	TrackTotal  int
	Year        string
}
