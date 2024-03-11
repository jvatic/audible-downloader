package audiometa

import (
	"bytes"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"reflect"

	"github.com/aler9/writerseeker"
	mp3TagLib "github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

// Save saves the corresponding IDTag to the audio file that it references and returns an error if the saving process fails
func (tag *IDTag) Save() error {
	fileType, err := GetFileType(tag.filePath)
	if err != nil {
		return err
	}
	if fileType == MP3 {
		return saveMP3(tag)
	} else if fileType == M4A || fileType == M4B || fileType == M4P || fileType == MP4 {
		return saveMP4(tag)
	} else if fileType == FLAC {
		return saveFLAC(tag)
	} else if fileType == OGG {
		return saveOGG(tag)
	}
	return fmt.Errorf("no method available for filetype:%s", tag.fileType)
}

func saveMP3(tag *IDTag) error {
	mp3Tag, err := mp3TagLib.Open(tag.filePath, mp3TagLib.Options{Parse: true})
	if err != nil {
		return err
	}
	defer mp3Tag.Close()
	fields := reflect.VisibleFields(reflect.TypeOf(*tag))
	for _, field := range fields {
		fieldName := field.Name
		if fieldName == "albumArt" {
			if reflect.ValueOf(*tag).FieldByName(fieldName).IsNil() {
				mp3Tag.DeleteFrames(mp3TextFrames[fieldName])
				continue
			}
			buf := new(bytes.Buffer)
			if err := jpeg.Encode(buf, *tag.albumArt, nil); err == nil {
				mp3Tag.AddAttachedPicture(mp3TagLib.PictureFrame{
					Encoding:    mp3TagLib.EncodingUTF8,
					MimeType:    "image/jpeg",
					PictureType: mp3TagLib.PTFrontCover,
					Description: "Front cover",
					Picture:     buf.Bytes(),
				})
			}
			continue
		}
		textFrame := mp3TagLib.TextFrame{
			Encoding: mp3TagLib.EncodingUTF8,
			Text:     reflect.ValueOf(*tag).FieldByName(fieldName).String(),
		}
		mp3Tag.AddFrame(mp3TextFrames[fieldName], textFrame)
	}
	file, err := os.Open(tag.filePath)
	if err != nil {
		return err
	}
	originalSize, err := parseHeader(file)
	if err != nil {
		return err
	}
	ws := &writerseeker.WriterSeeker{}
	// Write tag in new file.
	if _, err = mp3Tag.WriteTo(ws); err != nil {
		return err
	}

	// Seek to a music part of original file.
	if _, err = file.Seek(originalSize, io.SeekStart); err != nil {
		return err
	}

	// Write to new file the music part.
	buf := getByteSlice(128 * 1024)
	defer putByteSlice(buf)
	if _, err = io.CopyBuffer(ws, file, buf); err != nil {
		return err
	}
	file.Close()
	ofile, err := os.OpenFile(tag.filePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer ofile.Close()
	if _, err = ofile.Write(ws.Bytes()); err != nil {
		return err
	}
	ofile.Close()
	return nil
}

func saveMP4(tag *IDTag) error {
	var delete MP4Delete
	fields := reflect.VisibleFields(reflect.TypeOf(*tag))
	for _, field := range fields {
		fieldName := field.Name
		if fieldName == "albumArt" && reflect.ValueOf(*tag).FieldByName(fieldName).IsNil() {
			delete = append(delete, fieldName)
			continue
		}
		if reflect.ValueOf(*tag).FieldByName(fieldName).String() == "" {
			delete = append(delete, fieldName)
		}
	}
	return writeMP4(tag.filePath, tag, delete)
}

func saveFLAC(tag *IDTag) error {
	f, err := flac.ParseFile(tag.filePath)
	if err != nil {
		return err
	}
	_, idx, err := extractFLACComment(tag.filePath)
	if err != nil {
		return err
	}
	cmts := flacvorbis.New()
	if err := cmts.Add(flacvorbis.FIELD_TITLE, tag.title); err != nil {
		return err
	}
	if err := cmts.Add(flacvorbis.FIELD_ALBUM, tag.album); err != nil {
		return err
	}
	if err := cmts.Add(flacvorbis.FIELD_ARTIST, tag.artist); err != nil {
		return err
	}
	if err := cmts.Add(flacvorbis.FIELD_GENRE, tag.genre); err != nil {
		return err
	}
	cmtsmeta := cmts.Marshal()
	if idx > 0 {
		f.Meta = removeFLACMetaBlock(f.Meta, idx)
		f.Meta = append(f.Meta, &cmtsmeta)
	} else {
		f.Meta = append(f.Meta, &cmtsmeta)
	}
	idx = getFLACPictureIndex(f.Meta)
	if idx > 0 {
		f.Meta = removeFLACMetaBlock(f.Meta, idx)
	}
	if tag.albumArt != nil {
		buf := new(bytes.Buffer)
		if err := jpeg.Encode(buf, *tag.albumArt, nil); err == nil {
			picture, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", buf.Bytes(), "image/jpeg")
			if err != nil {
				return err
			}
			picturemeta := picture.Marshal()
			f.Meta = append(f.Meta, &picturemeta)
		}

	}
	return f.Save(tag.filePath)
}

func saveOGG(tag *IDTag) error {
	if tag.codec == "vorbis" {
		return saveVorbisTags(tag)
	} else if tag.codec == "opus" {
		return saveOpusTags(tag)
	}
	return errors.New("codec not supported for OGG")
}
