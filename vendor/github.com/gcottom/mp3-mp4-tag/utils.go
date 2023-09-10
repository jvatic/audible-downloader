package mp3mp4tag

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

func getFileType(filepath string) (*string, error) {
	fileTypeArr := strings.Split(filepath, ".")
	lastIndex := len(fileTypeArr) - 1
	fileType := fileTypeArr[lastIndex]
	fileType = strings.ToLower(fileType)
	if fileType == "mp3" || fileType == "m4p" || fileType == "m4a" || fileType == "m4b" || fileType == "mp4" || fileType == "flac" || fileType == "ogg" {
		return &fileType, nil
	} else {
		return nil, errors.New("Format: Unsupported Format: " + fileType)
	}
}

func getInt(b []byte) int {
	var n int
	for _, x := range b {
		n = n << 8
		n |= int(x)
	}
	return n
}
func readInt(r io.Reader, n uint) (int, error) {
	b, err := readBytes(r, n)
	if err != nil {
		return 0, err
	}
	return getInt(b), nil
}

func readUint(r io.Reader, n uint) (uint, error) {
	x, err := readInt(r, n)
	if err != nil {
		return 0, err
	}
	return uint(x), nil
}

// readBytesMaxUpfront is the max up-front allocation allowed
const readBytesMaxUpfront = 10 << 20 // 10MB

func readBytes(r io.Reader, n uint) ([]byte, error) {
	if n > readBytesMaxUpfront {
		b := &bytes.Buffer{}
		if _, err := io.CopyN(b, r, int64(n)); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}

	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func readString(r io.Reader, n uint) (string, error) {
	b, err := readBytes(r, n)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
func readUint32LittleEndian(r io.Reader) (uint32, error) {
	b, err := readBytes(r, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}
func encodeUint32(n uint32) []byte {
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.BigEndian, n); err != nil {
		panic(err)
	}
	return buf.Bytes()
}