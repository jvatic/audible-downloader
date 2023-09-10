package mp3mp4tag

import (
	"encoding/binary"
)

// The MIME type as defined in RFC 3534.
const MIMEType = "application/ogg"

const headerSize = 27

// max segment size
const maxSegSize = 255

// max sequence-of-segments size in a page
const mps = maxSegSize * 255

// == 65307, per the RFC
const maxPageSize = headerSize + maxSegSize + mps

// The byte order of integers in ogg page headers.
var byteOrder = binary.LittleEndian

type oggPageHeader struct {
	Magic           [4]byte // 0-3, always == "OggS"
	Version         byte    // 4, always == 0
	Flags           byte    // 5
	GranulePosition int64   // 6-13, codec-specific
	SerialNumber    uint32  // 14-17, associated with a logical stream
	SequenceNumber  uint32  // 18-21, sequence number of page in packet
	CRC             uint32  // 22-25
	Segments        byte    // 26
}

const (
	// Continuation of packet
	COP byte = 1 << iota
	// Beginning of stream
	BOS = 1 << iota
	// End of stream
	EOS = 1 << iota
)

// "unreflected" crc used by libogg
func crc32(p []byte) uint32 {
	crcTable := oggCRC32Poly04c11db7
	c := uint32(0)
	for _, n := range p {
		c = crcTable[byte(c>>24)^n] ^ (c << 8)
	}
	return c
}
