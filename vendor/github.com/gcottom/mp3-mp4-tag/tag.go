package mp3mp4tag

import (
	"bytes"
	"image"
	"os"
)

// Opens the ID tag for the corresponding file as long as it is a supported filetype
// Use the OpenTag command and you will be able to access all metadata associated with the file
func OpenTag(filepath string) (*IDTag, error) {
	tag, err := parse(filepath)
	if err != nil {
		return nil, err
	} else {
		return tag, nil
	}
}

// This operation saves the corresponding IDTag to the mp3/mp4 file that it references and returns an error if the saving process fails
func SaveTag(tag *IDTag) error {
	err := tag.Save()
	if err != nil {
		return err
	} else {
		return nil
	}
}

// clears all tags except the fileUrl tag which is used to reference the file, takes an optional parameter "preserveUnkown": when this is true passThroughMap is not cleared and unknown tags are preserved
func (tag *IDTag) ClearAllTags(preserveUnknown ...bool) {
	tag.artist = ""
	tag.albumArtist = ""
	tag.album = ""
	tag.albumArt = nil
	tag.comments = ""
	tag.composer = ""
	tag.genre = ""
	tag.title = ""
	tag.year = ""
	tag.bpm = ""

	tag.idTagExtended.copyrightMsg = ""
	tag.idTagExtended.date = ""
	tag.idTagExtended.encodedBy = ""
	tag.idTagExtended.lyricist = ""
	tag.idTagExtended.fileType = ""
	tag.idTagExtended.language = ""
	tag.idTagExtended.length = ""
	tag.idTagExtended.partOfSet = ""
	tag.idTagExtended.publisher = ""

	preserve := false
	if len(preserveUnknown) == 0 {
		preserve = false
	} else {
		preserve = preserveUnknown[0]
	}
	if !preserve {
		tag.passThroughMap = make(map[string]string)
	}

}

// Get the artist for a tag
func (tag *IDTag) Artist() string {
	return tag.artist
}

// Set the artist for a tag
func (tag *IDTag) SetArtist(artist string) {
	tag.artist = artist
}

// Get the album artist for a tag
func (tag *IDTag) AlbumArtist() string {
	return tag.albumArtist
}

// Set teh album artist for a tag
func (tag *IDTag) SetAlbumArtist(albumArtist string) {
	tag.albumArtist = albumArtist
}

// Get the album for a tag
func (tag *IDTag) Album() string {
	return tag.album
}

// Set the album for a tag
func (tag *IDTag) SetAlbum(album string) {
	tag.album = album
}

// Get the commnets for a tag
func (tag *IDTag) Comments() string {
	return tag.comments
}

// Set the comments for a tag
func (tag *IDTag) SetComments(comments string) {
	tag.comments = comments
}

// Get the composer for a tag
func (tag *IDTag) Composer() string {
	return tag.composer
}

// Set the composer for a tag
func (tag *IDTag) SetComposer(composer string) {
	tag.composer = composer
}

// Get the genre for a tag
func (tag *IDTag) Genre() string {
	return tag.genre
}

// Set the genre for a tag
func (tag *IDTag) SetGenre(genre string) {
	tag.genre = genre
}

// Get the title for a tag
func (tag *IDTag) Title() string {
	return tag.title
}

// Set the title for a tag
func (tag *IDTag) SetTitle(title string) {
	tag.title = title
}

// Get the year for a tag
func (tag *IDTag) Year() string {
	return tag.year
}

// Set the year for a tag
func (tag *IDTag) SetYear(year string) {
	tag.year = year
}

// Get the BPM for a tag
func (tag *IDTag) BPM() string {
	return tag.bpm
}

// Set the BPM for a tag
func (tag *IDTag) SetBPM(bpm string) {
	tag.bpm = bpm
}

// Get the Copyright Messgae for a tag
func (tag *IDTag) CopyrightMsg() string {
	return tag.idTagExtended.copyrightMsg
}

// Set the Copyright Message for a tag
func (tag *IDTag) SetCopyrightMsg(copyrightMsg string) {
	tag.idTagExtended.copyrightMsg = copyrightMsg
}

// Get the date for a tag
func (tag *IDTag) Date() string {
	return tag.idTagExtended.date
}

// Set the date for a tag
func (tag *IDTag) SetDate(date string) {
	tag.idTagExtended.date = date
}

// Get who encoded the tag
func (tag *IDTag) EncodedBy() string {
	return tag.idTagExtended.encodedBy
}

// Set who encoded the tag
func (tag *IDTag) SetEncodedBy(encodedBy string) {
	tag.idTagExtended.encodedBy = encodedBy
}

// Get the lyricist for the tag
func (tag *IDTag) Lyricist() string {
	return tag.idTagExtended.lyricist
}

// Set the lyricist for the tag
func (tag *IDTag) SetLyricist(lyricist string) {
	tag.idTagExtended.lyricist = lyricist
}

// Get the filetype of the tag
func (tag *IDTag) FileType() string {
	return tag.idTagExtended.fileType
}

// Set the filtype of the tag
func (tag *IDTag) SetFileType(fileType string) {
	tag.idTagExtended.fileType = fileType
}

// Get the language of the tag
func (tag *IDTag) Language() string {
	return tag.idTagExtended.language
}

// Set the lanuguage of the tag
func (tag *IDTag) SetLanguage(language string) {
	tag.idTagExtended.language = language
}

// Get the langth of the tag
func (tag *IDTag) Length() string {
	return tag.idTagExtended.length
}

// Set the length of the tag
func (tag *IDTag) SetLength(length string) {
	tag.idTagExtended.length = length
}

// Get if tag is part of a set
func (tag *IDTag) PartOfSet() string {
	return tag.idTagExtended.partOfSet
}

// Set if the tag is part of a set
func (tag *IDTag) SetPartOfSet(partOfSet string) {
	tag.idTagExtended.partOfSet = partOfSet
}

// Get publisher for the tag
func (tag *IDTag) Publisher() string {
	return tag.idTagExtended.publisher
}

// Set publihser for the tag
func (tag *IDTag) SetPublisher(publisher string) {
	tag.idTagExtended.publisher = publisher
}

// Get all additional (unmapped) tags
func (tag *IDTag) AdditionalTags() map[string]string {
	return tag.passThroughMap
}

// Set an additional (unmapped) tag
func (tag *IDTag) SetAdditionalTag(id string, value string) {
	tag.passThroughMap[id] = value
}

// Set a map for all additional tags
func (tag *IDTag) SetAllAdditionalTags(set map[string]string) {
	tag.passThroughMap = set
}

func (tag *IDTag) ClearAllAdditionalTags() {
	tag.passThroughMap = make(map[string]string)
}

// Get the album art for the tag as an *image.Image
func (tag *IDTag) AlbumArt() *image.Image {
	return tag.albumArt
}

// Set the album art by passing a byte array for the album art
func (tag *IDTag) SetAlbumArtFromByteArray(albumArt []byte) error {
	img, _, err := image.Decode(bytes.NewReader(albumArt))
	if err != nil {
		return err
	} else {
		tag.albumArt = &img
		return nil
	}
}

// Set the album art by passing an *image.Image as the album art
func (tag *IDTag) SetAlbumArtFromImage(albumArt *image.Image) {
	tag.albumArt = albumArt
}

// Set the album art by passing a filepath as a string
func (tag *IDTag) SetAlbumArtFromFilePath(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	tag.albumArt = &img
	return nil
}
