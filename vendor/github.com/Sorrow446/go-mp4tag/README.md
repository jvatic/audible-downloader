# go-mp4tag
MP4 tagging library & CLI tagger written in Go.    
[Windows, Linux, macOS and Android binaries](https://github.com/Sorrow446/go-mp4tag/releases)


## Library
### Setup
```
go get github.com/Sorrow446/go-mp4tag
```
```go
import "github.com/Sorrow446/go-mp4tag"
```

### Usage Examples
```go
	tags := &mp4tag.Tags{
		Album: "album",
		AlbumArtist: "album artist",
		Title:       "title",
		TrackNumber: 1,
		TrackTotal:  20,
		Genre: "genre",
		DiskNumber:  3,
		DiskTotal:   10,
		Comment:     "comment",
	}
	err := mp4tag.Write("1.m4a", tags)
	if err != nil {
		panic(err)
	}
```
 Write album, album artist, title, track number, track total, genre, disk number, disk total, and comment tags.
 
 
 ```go
	tags := &mp4tag.Tags{
		Custom: map[string]string{
			"CUSTOMFIELD": "custom field",
			"CUSTOMFIELD2": "custom field 2",
		},
		Delete: []string{"cover", "genre"},
	}
	err := mp4tag.Write("1.m4a", tags)
	if err != nil {
		panic(err)
	}
```
Write two custom fields named `CUSTOMFIELD` and `CUSTOMFIELD2`, delete genre tag, and remove cover.


```go
	coverBytes, err := ioutil.ReadFile("cover.jpg")
	if err != nil {
		panic(err)
	}
	tags := &mp4tag.Tags{
		Cover: coverBytes,
	}
	err = mp4tag.Write("1.m4a", tags)
	if err != nil {
		panic(err)
	}
```
Write cover from `cover.jpg`.

```go
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
```
iTunes-style metadata only.       
Delete strings: album, albumartist, artist, comment, composer, copyright, cover, disk, genre, label, title, track, year.    
Custom tag deletion is not implemented yet.

## CLI
go-mp4tag also has a CLI version if you'd like to call it outside of Go.
```
Usage: mp4tag_x64.exe [--album ALBUM] [--albumArtist ALBUMARTIST] [--artist ARTIST] [--comment COMMENT] [--composer COMPOSER] [--copyright COPYRIGHT] [--cover COVER] [--custom CUSTOM] [--delete DELETE] [--diskNumber DISKNUMBER] [--diskTotal DISKTOTAL] [--genre GENRE] [--label LABEL] [--title TITLE] [--trackNumber TRACKNUMBER] [--trackTotal TRACKTOTAL] [--year YEAR] FILEPATH

Positional arguments:
  FILEPATH               Path of file to write to.

Options:
  --album ALBUM          Write album tag.
  --albumArtist ALBUMARTIST
                         Write album artist tag.
  --artist ARTIST        Write artist tag.
  --comment COMMENT      Write comment tag.
  --composer COMPOSER    Write composer tag.
  --copyright COPYRIGHT
                         Write copyright tag.
  --cover COVER          Path of cover to write. JPEG is recommended.
  --custom CUSTOM        Write custom tags. Multiple tags with the same field name can be written.
                         Example: "--custom MYCUSTOMFIELD1=value1 MYCUSTOMFIELD2=value2"
  --delete DELETE, -d DELETE
                         Tags to delete.
                         Options: album, albumartist, artist, comment, composer, copyright, cover, disk, genre, label, title, track, year.
                         Example: "-d album albumartist"
  --diskNumber DISKNUMBER
                         Write disk number tag.
  --diskTotal DISKTOTAL
                         Write disk total tag. Can't be written without disk number tag.
  --genre GENRE          Write genre tag.
  --label LABEL          Write label tag.
  --title TITLE          Write title tag.
  --trackNumber TRACKNUMBER
                         Write track number tag.
  --trackTotal TRACKTOTAL
                         Write track total tag. Can't be written without track number tag.
  --year YEAR            Write year tag.
  --help, -h             display this help and exit
  ```
  You must use double quotes for values with spaces in.
  
  `mp4tag_x64.exe 1.m4a --artist artist --albumArtist "album artist"`    
  Write artist and album artist tags.
  
  `mp4tag_x64.exe 1.m4a --cover cover.jpg`    
  Write cover from `cover.jpg`
  
  `mp4tag_x64.exe 1.m4a --custom "MY CUSTOM FIELD WITH SPACES"=value1 MYCUSTOMFIELD1="value with spaces" -d cover genre"`    
  Write two custom fields named `MY CUSTOM FIELD WITH SPACES`, and `MYCUSTOMFIELD1`, delete genre tag, and remove cover.

## Thank you
 go-mp4tag relies heavily on abema's go-mp4 library.

## Disclaimer
Although go-mp4tag has been thoroughly tested, I will not be responsible for the very tiny chance of any corruption to your MP4 files.
