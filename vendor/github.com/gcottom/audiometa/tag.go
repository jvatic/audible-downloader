package audiometa

import (
	"bytes"
	"image"
	"os"
)

//OpenTag Opens the ID tag for the corresponding file as long as it is a supported filetype
//Use the OpenTag command and you will be able to access all metadata associated with the file
func OpenTag(filepath string) (*IDTag, error) {
	tag, err := parse(filepath)
	if err != nil {
		return nil, err
	} else {
		return tag, nil
	}
}

//SaveTag saves the corresponding IDTag to the audio file that it references and returns an error if the saving process fails
func SaveTag(tag *IDTag) error {
	err := tag.Save()
	if err != nil {
		return err
	} else {
		return nil
	}
}

//ClearAllTags clears all tags except the fileUrl tag which is used to reference the file, takes an optional parameter "preserveUnkown": when this is true passThroughMap is not cleared and unknown tags are preserved
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

//Artist gets the artist for a tag
func (tag *IDTag) Artist() string {
	return tag.artist
}

//SetArtist sets the artist for a tag
func (tag *IDTag) SetArtist(artist string) {
	tag.artist = artist
}

//AlbumArtist gets the album artist for a tag
func (tag *IDTag) AlbumArtist() string {
	return tag.albumArtist
}

//SetAlbumArtist sets the album artist for a tag
func (tag *IDTag) SetAlbumArtist(albumArtist string) {
	tag.albumArtist = albumArtist
}

//Album gets the album for a tag
func (tag *IDTag) Album() string {
	return tag.album
}

//SetAlbum sets the album for a tag
func (tag *IDTag) SetAlbum(album string) {
	tag.album = album
}

//Comments gets the comments for a tag
func (tag *IDTag) Comments() string {
	return tag.comments
}

//SetComments sets the comments for a tag
func (tag *IDTag) SetComments(comments string) {
	tag.comments = comments
}

//Composer gets the composer for a tag
func (tag *IDTag) Composer() string {
	return tag.composer
}

//SetComposer sets the composer for a tag
func (tag *IDTag) SetComposer(composer string) {
	tag.composer = composer
}

//Genre gets the genre for a tag
func (tag *IDTag) Genre() string {
	return tag.genre
}

//SetGenre sets the genre for a tag
func (tag *IDTag) SetGenre(genre string) {
	tag.genre = genre
}

//Title gets the title for a tag
func (tag *IDTag) Title() string {
	return tag.title
}

//SetTitle sets the title for a tag
func (tag *IDTag) SetTitle(title string) {
	tag.title = title
}

//Year gets the year for a tag as a string
func (tag *IDTag) Year() string {
	return tag.year
}

//SetYear sets the year for a tag
func (tag *IDTag) SetYear(year string) {
	tag.year = year
}

//BPM gets the BPM for a tag as a string
func (tag *IDTag) BPM() string {
	return tag.bpm
}

//SetBPM sets the BPM for a tag
func (tag *IDTag) SetBPM(bpm string) {
	tag.bpm = bpm
}

//CopyrightMs gets the Copyright Messgae for a tag
func (tag *IDTag) CopyrightMsg() string {
	return tag.idTagExtended.copyrightMsg
}

//SetCopyrightMsg sets the Copyright Message for a tag
func (tag *IDTag) SetCopyrightMsg(copyrightMsg string) {
	tag.idTagExtended.copyrightMsg = copyrightMsg
}

//Date gets the date for a tag as a string
func (tag *IDTag) Date() string {
	return tag.idTagExtended.date
}

//SetDate sets the date for a tag
func (tag *IDTag) SetDate(date string) {
	tag.idTagExtended.date = date
}

//EncodedBy gets who encoded the tag
func (tag *IDTag) EncodedBy() string {
	return tag.idTagExtended.encodedBy
}

//SetEncodedBy sets who encoded the tag
func (tag *IDTag) SetEncodedBy(encodedBy string) {
	tag.idTagExtended.encodedBy = encodedBy
}

//Lyricist gets the lyricist for the tag
func (tag *IDTag) Lyricist() string {
	return tag.idTagExtended.lyricist
}

//SetLyricist sets the lyricist for the tag
func (tag *IDTag) SetLyricist(lyricist string) {
	tag.idTagExtended.lyricist = lyricist
}

//FileType gets the filetype of the tag
func (tag *IDTag) FileType() string {
	return tag.idTagExtended.fileType
}

//SetFileType sets the filtype of the tag
func (tag *IDTag) SetFileType(fileType string) {
	tag.idTagExtended.fileType = fileType
}

//Language gets the language of the tag
func (tag *IDTag) Language() string {
	return tag.idTagExtended.language
}

//SetLanguage sets the lanuguage of the tag
func (tag *IDTag) SetLanguage(language string) {
	tag.idTagExtended.language = language
}

//Length gets the length of the audio file
func (tag *IDTag) Length() string {
	return tag.idTagExtended.length
}

//SetLength sets the length of the audio file
func (tag *IDTag) SetLength(length string) {
	tag.idTagExtended.length = length
}

//PartOfSet gets if the track is part of a set
func (tag *IDTag) PartOfSet() string {
	return tag.idTagExtended.partOfSet
}

//SetPartOfSet sets if the track is part of a set
func (tag *IDTag) SetPartOfSet(partOfSet string) {
	tag.idTagExtended.partOfSet = partOfSet
}

//Publisher gets the publisher for the tag
func (tag *IDTag) Publisher() string {
	return tag.idTagExtended.publisher
}

//SetPublisher sets the publisher for the tag
func (tag *IDTag) SetPublisher(publisher string) {
	tag.idTagExtended.publisher = publisher
}

//AdditionalTags gets all additional (unmapped) tags
func (tag *IDTag) AdditionalTags() map[string]string {
	return tag.passThroughMap
}

//SetAdditionalTag sets an additional (unmapped) tag taking an id and value (id,value) (ogg only) 
func (tag *IDTag) SetAdditionalTag(id string, value string) {
	tag.passThroughMap[id] = value
}

//SetAllAdditionalTags takes a map of all additional tags and sets all additional tags
func (tag *IDTag) SetAllAdditionalTags(set map[string]string) {
	tag.passThroughMap = set
}
//ClearAllAdditionalTags removes all additional tags from the tag set
func (tag *IDTag) ClearAllAdditionalTags() {
	tag.passThroughMap = make(map[string]string)
}

//Album art gets the album art for the tag as an *image.Image
func (tag *IDTag) AlbumArt() *image.Image {
	return tag.albumArt
}

//SetAlbumArtFromByteArray sets the album art by passing a byte array for the album art
func (tag *IDTag) SetAlbumArtFromByteArray(albumArt []byte) error {
	img, _, err := image.Decode(bytes.NewReader(albumArt))
	if err != nil {
		return err
	} else {
		tag.albumArt = &img
		return nil
	}
}

//SetAlbumArtFromImage sets the album art by passing an *image.Image as the album art
func (tag *IDTag) SetAlbumArtFromImage(albumArt *image.Image) {
	tag.albumArt = albumArt
}

//SetAlbumArtFromFilePath sets the album art by passing a filepath as a string
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
