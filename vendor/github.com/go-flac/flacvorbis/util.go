package flacvorbis

import (
	"bytes"
	"encoding/binary"
	"io"
)

func encodeUint32(n uint32) []byte {
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.LittleEndian, n); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func readUint32(r io.Reader) (res uint32, err error) {
	err = binary.Read(r, binary.LittleEndian, &res)
	return
}

func packStr(w io.Writer, s string) {
	data := []byte(s)
	w.Write(encodeUint32(uint32(len(data))))
	w.Write(data)
}
