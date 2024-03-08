package audiometa

import (
	"bytes"
	"image"
	"os"
)

// OpenTag Opens the ID tag for the corresponding file as long as it is a supported filetype
// Use the OpenTag command and you will be able to access all metadata associated with the file
func OpenTag(filepath string) (*IDTag, error) {
	return parse(filepath)
}

// SaveTag saves the corresponding IDTag to the audio file that it references and returns an error if the saving process fails
func SaveTag(tag *IDTag) error {
	return tag.Save()
}

// ClearAllTags clears all tags except the fileUrl tag which is used to reference the file, takes an optional parameter "preserveUnkown": when this is true passThroughMap is not cleared and unknown tags are preserved
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
	tag.copyrightMsg = ""
	tag.date = ""
	tag.encodedBy = ""
	tag.lyricist = ""
	tag.fileType = ""
	tag.language = ""
	tag.length = ""
	tag.partOfSet = ""
	tag.publisher = ""

	preserve := false
	if len(preserveUnknown) != 0 {
		preserve = preserveUnknown[0]
	}
	if !preserve {
		tag.PassThrough = make(map[string]string)
	}

}

// Artist gets the artist for a tag
func (tag *IDTag) Artist() string {
	return tag.artist
}

// SetArtist sets the artist for a tag
func (tag *IDTag) SetArtist(artist string) {
	tag.artist = artist
}

// AlbumArtist gets the album artist for a tag
func (tag *IDTag) AlbumArtist() string {
	return tag.albumArtist
}

// SetAlbumArtist sets the album artist for a tag
func (tag *IDTag) SetAlbumArtist(albumArtist string) {
	tag.albumArtist = albumArtist
}

// Album gets the album for a tag
func (tag *IDTag) Album() string {
	return tag.album
}

// SetAlbum sets the album for a tag
func (tag *IDTag) SetAlbum(album string) {
	tag.album = album
}

// Comments gets the comments for a tag
func (tag *IDTag) Comments() string {
	return tag.comments
}

// SetComments sets the comments for a tag
func (tag *IDTag) SetComments(comments string) {
	tag.comments = comments
}

// Composer gets the composer for a tag
func (tag *IDTag) Composer() string {
	return tag.composer
}

// SetComposer sets the composer for a tag
func (tag *IDTag) SetComposer(composer string) {
	tag.composer = composer
}

// Genre gets the genre for a tag
func (tag *IDTag) Genre() string {
	return tag.genre
}

// SetGenre sets the genre for a tag
func (tag *IDTag) SetGenre(genre string) {
	tag.genre = genre
}

// Title gets the title for a tag
func (tag *IDTag) Title() string {
	return tag.title
}

// SetTitle sets the title for a tag
func (tag *IDTag) SetTitle(title string) {
	tag.title = title
}

// Year gets the year for a tag as a string
func (tag *IDTag) Year() string {
	return tag.year
}

// SetYear sets the year for a tag
func (tag *IDTag) SetYear(year string) {
	tag.year = year
}

// BPM gets the BPM for a tag as a string
func (tag *IDTag) BPM() string {
	return tag.bpm
}

// SetBPM sets the BPM for a tag
func (tag *IDTag) SetBPM(bpm string) {
	tag.bpm = bpm
}

// CopyrightMs gets the Copyright Messgae for a tag
func (tag *IDTag) CopyrightMsg() string {
	return tag.copyrightMsg
}

// SetCopyrightMsg sets the Copyright Message for a tag
func (tag *IDTag) SetCopyrightMsg(copyrightMsg string) {
	tag.copyrightMsg = copyrightMsg
}

// Date gets the date for a tag as a string
func (tag *IDTag) Date() string {
	return tag.date
}

// SetDate sets the date for a tag
func (tag *IDTag) SetDate(date string) {
	tag.date = date
}

// EncodedBy gets who encoded the tag
func (tag *IDTag) EncodedBy() string {
	return tag.encodedBy
}

// SetEncodedBy sets who encoded the tag
func (tag *IDTag) SetEncodedBy(encodedBy string) {
	tag.encodedBy = encodedBy
}

// Lyricist gets the lyricist for the tag
func (tag *IDTag) Lyricist() string {
	return tag.lyricist
}

// SetLyricist sets the lyricist for the tag
func (tag *IDTag) SetLyricist(lyricist string) {
	tag.lyricist = lyricist
}

// FileType gets the filetype of the tag
func (tag *IDTag) FileType() string {
	return tag.fileType
}

// SetFileType sets the filtype of the tag
func (tag *IDTag) SetFileType(fileType string) {
	tag.fileType = fileType
}

// Language gets the language of the tag
func (tag *IDTag) Language() string {
	return tag.language
}

// SetLanguage sets the lanuguage of the tag
func (tag *IDTag) SetLanguage(language string) {
	tag.language = language
}

// Length gets the length of the audio file
func (tag *IDTag) Length() string {
	return tag.length
}

// SetLength sets the length of the audio file
func (tag *IDTag) SetLength(length string) {
	tag.length = length
}

// PartOfSet gets if the track is part of a set
func (tag *IDTag) PartOfSet() string {
	return tag.partOfSet
}

// SetPartOfSet sets if the track is part of a set
func (tag *IDTag) SetPartOfSet(partOfSet string) {
	tag.partOfSet = partOfSet
}

// Publisher gets the publisher for the tag
func (tag *IDTag) Publisher() string {
	return tag.publisher
}

// SetPublisher sets the publisher for the tag
func (tag *IDTag) SetPublisher(publisher string) {
	tag.publisher = publisher
}

// AdditionalTags gets all additional (unmapped) tags
func (tag *IDTag) AdditionalTags() map[string]string {
	return tag.PassThrough
}

// SetAdditionalTag sets an additional (unmapped) tag taking an id and value (id,value) (ogg only)
func (tag *IDTag) SetAdditionalTag(id string, value string) {
	tag.PassThrough[id] = value
}

// SetAlbumArtFromByteArray sets the album art by passing a byte array for the album art
func (tag *IDTag) SetAlbumArtFromByteArray(albumArt []byte) error {
	img, _, err := image.Decode(bytes.NewReader(albumArt))
	if err != nil {
		return err
	}
	tag.albumArt = &img
	return nil
}

// SetAlbumArtFromImage sets the album art by passing an *image.Image as the album art
func (tag *IDTag) SetAlbumArtFromImage(albumArt *image.Image) {
	tag.albumArt = albumArt
}

// SetAlbumArtFromFilePath sets the album art by passing a filepath as a string
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
