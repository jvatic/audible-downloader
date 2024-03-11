package audiometa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
)

// OggDecoder is a structure that facilitates the page-by-page decoding of an ogg stream.
type oggDecoder struct {
	// lenbuf acts as a buffer for packet lengths and helps avoid allocating (maxSegSize is the maximum per page)
	// r is an io.Reader used to read the data
	// buf is a byte array of maximum page size
	lenbuf [maxSegSize]int
	r      io.Reader
	buf    [maxPageSize]byte
}

// NewOggDecoder is a constructor that initializes an ogg Decoder.
func newOggDecoder(r io.Reader) *oggDecoder {
	// returns a new instance of Decoder
	return &oggDecoder{r: r}
}

// Page struct represents a logical unit of an ogg page.
type oggPage struct {
	// Type is a bitmask of COP, BOS, and/or EOS.
	Type byte
	// Serial represents the bitstream serial number.
	Serial uint32
	// Granule represents the granule position, its interpretation depends on the encapsulated codec.
	Granule int64
	// Packets are the actual packet data.
	// If Type & COP != 0, the first element is
	// a continuation of the previous page's last packet.
	Packets [][]byte
}

// errBadSegs error is thrown when an attempt is made to decode a page with a segment table size less than 1.
var errBadSegs = errors.New("invalid segment table size")

// ErrBadCrc error occurs when the CRC field in an ogg page doesn't match the CRC calculated by the Decoder.
type errBadCrc struct {
	Found    uint32
	Expected uint32
}

// Error function to return a formatted error message
func (bc errBadCrc) Error() string {
	// returning a formatted error message with received and expected CRC values
	return "crc error in packet: invalid " + strconv.FormatInt(int64(bc.Found), 16) +
		", expected " + strconv.FormatInt(int64(bc.Expected), 16)
}

// oggs is a byte slice representing the sequence 'OggS'
var oggs = []byte{'O', 'g', 'g', 'S'}

// / Decode reads from the Reader of the Decoder until the next ogg page is found, then decodes and returns the Page or an error.
// An io.EOF error can be returned if that's what the Reader returns.
//
// The memory for the returned Page's Packets' bytes is managed by the Decoder.
// It can be overwritten by subsequent calls to Decode.
//
// It's safe to call Decode concurrently on different Decoders, provided their Readers are distinct.
// Otherwise, the outcome is not defined.
func (d *oggDecoder) decodeOgg() (oggPage, error) {
	hbuf := d.buf[0:headerSize]
	b := 0
	for {
		_, err := io.ReadFull(d.r, hbuf[b:])
		if err != nil {
			return oggPage{}, err
		}

		i := bytes.Index(hbuf, oggs)
		if i == 0 {
			break
		}

		if i < 0 {
			const n = headerSize
			if hbuf[n-1] == 'O' {
				i = n - 1
			} else if hbuf[n-2] == 'O' && hbuf[n-1] == 'g' {
				i = n - 2
			} else if hbuf[n-3] == 'O' && hbuf[n-2] == 'g' && hbuf[n-1] == 'g' {
				i = n - 3
			}
		}

		if i > 0 {
			b = copy(hbuf, hbuf[i:])
		}
	}

	var h oggPageHeader
	_ = binary.Read(bytes.NewBuffer(hbuf), byteOrder, &h)

	if h.Segments < 1 {
		return oggPage{}, errBadSegs
	}

	nsegs := int(h.Segments)
	segtbl := d.buf[headerSize : headerSize+nsegs]
	if _, err := io.ReadFull(d.r, segtbl); err != nil {
		return oggPage{}, err
	}

	// A page may encompass multiple packets. Hence, we extract their lengths from the table at this stage,
	// and subsequently segment the payload after reading it.

	packetlens := d.lenbuf[0:0]
	payloadlen := 0
	more := false
	for _, l := range segtbl {
		if more {
			packetlens[len(packetlens)-1] += int(l)
		} else {
			packetlens = append(packetlens, int(l))
		}

		more = l == maxSegSize
		payloadlen += int(l)
	}

	payload := d.buf[headerSize+nsegs : headerSize+nsegs+payloadlen]
	if _, err := io.ReadFull(d.r, payload); err != nil {
		return oggPage{}, err
	}

	page := d.buf[0 : headerSize+nsegs+payloadlen]
	// Clear out existing crc before calculating it
	page[22] = 0
	page[23] = 0
	page[24] = 0
	page[25] = 0
	crc := crc32(page)
	if crc != h.CRC {
		return oggPage{}, errBadCrc{h.CRC, crc}
	}

	packets := make([][]byte, len(packetlens))
	s := 0
	for i, l := range packetlens {
		packets[i] = payload[s : s+l]
		s += l
	}

	return oggPage{h.Flags, h.SerialNumber, h.GranulePosition, packets}, nil
}
