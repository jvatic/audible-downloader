package flacvorbis

import "errors"

var (
	ErrorNotVorbisComment = errors.New("Not a vorbis comment metadata block")
	ErrorUnexpEof         = errors.New("Unexpected end of stream")
	ErrorMalformedComment = errors.New("Malformed comment")
	ErrorInvalidFieldName = errors.New("Malformed Field Name")
)
