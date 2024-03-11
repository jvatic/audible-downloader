package audiometa

import (
	"bytes"
	"encoding/binary"
	"io"
)

// Encoder converts raw bytes into an ogg stream.
type oggEncoder struct {
	serial uint32
	page   uint32
	dummy  [1][]byte
	w      io.Writer
	buf    [maxPageSize]byte
}

// NewEncoder initializes an ogg encoder with a given serial ID.
// When using multiple Encoders for multiplexed logical streams, ensure unique IDs.
// Encode streams as per ogg RFC for Grouping and Chaining.
func newOggEncoder(id uint32, w io.Writer) *oggEncoder {
	return &oggEncoder{serial: id, w: w}
}

// EncodeBOS writes a beginning-of-stream packet to the ogg stream with a given granule position.
// Large packets are split across multiple pages with continuation-of-packet flag set.
// Packets can be empty or nil, resulting in a single segment of size 0.
func (w *oggEncoder) encodeBOS(granule int64, packets [][]byte) error {
	if len(packets) == 0 {
		packets = w.dummy[:]
	}
	return w.writePackets(BOS, granule, packets)
}

// Encode writes a data packet to the ogg stream with a given granule position.
// Large packets are split across multiple pages with continuation-of-packet flag set.
// Packets can be empty or nil, resulting in a single segment of size 0.
func (w *oggEncoder) encode(granule int64, packets [][]byte) error {
	if len(packets) == 0 {
		packets = w.dummy[:]
	}
	return w.writePackets(0, granule, packets)
}

// EncodeEOS writes a end-of-stream packet to the ogg stream.
// Packets can be empty or nil, resulting in a single segment of size 0.
func (w *oggEncoder) encodeEOS(granule int64, packets [][]byte) error {
	if len(packets) == 0 {
		packets = w.dummy[:]
	}
	return w.writePackets(EOS, granule, packets)
}

func (w *oggEncoder) writePackets(kind byte, granule int64, packets [][]byte) error {
	h := oggPageHeader{
		Magic:           [4]byte{'O', 'g', 'g', 'S'},
		Flags:           kind,
		SerialNumber:    w.serial,
		GranulePosition: granule,
	}

	segtbl, car, cdr := w.segmentize(payload{packets[0], packets[1:], nil})
	if err := w.writePage(&h, segtbl, car); err != nil {
		return err
	}

	h.Flags |= COP
	for len(cdr.leftover) > 0 {
		segtbl, car, cdr = w.segmentize(cdr)
		if err := w.writePage(&h, segtbl, car); err != nil {
			return err
		}
	}

	return nil
}

func (w *oggEncoder) writePage(h *oggPageHeader, segtbl []byte, pay payload) error {
	h.SequenceNumber = w.page
	w.page++
	h.Segments = byte(len(segtbl))
	hb := bytes.NewBuffer(w.buf[0:0:cap(w.buf)])
	_ = binary.Write(hb, byteOrder, h)

	hb.Write(segtbl)

	hb.Write(pay.leftover)
	for _, p := range pay.packets {
		hb.Write(p)
	}
	hb.Write(pay.rightover)

	bb := hb.Bytes()
	crc := crc32(bb)
	_ = binary.Write(bytes.NewBuffer(bb[22:22:26]), byteOrder, crc)

	_, err := hb.WriteTo(w.w)
	return err
}

// payload represents a potentially split group of packets.
// ASCII example (each letter run represents one packet):
// Page 1 (left): [aaaabbbbccccd], Page 2 (right): [dddeeeffff]
type payload struct {
	leftover  []byte
	packets   [][]byte
	rightover []byte
}

// segmentize calculates the lacing values for the segment table based on given packets.
// Returns the segment table, the payload for the current page and any leftover payload.
func (w *oggEncoder) segmentize(pay payload) ([]byte, payload, payload) {
	segtbl := w.buf[headerSize : headerSize+maxSegSize]
	i := 0

	s255s := len(pay.leftover) / maxSegSize
	rem := len(pay.leftover) % maxSegSize
	for i < len(segtbl) && s255s > 0 {
		segtbl[i] = maxSegSize
		i++
		s255s--
	}
	if i < maxSegSize {
		segtbl[i] = byte(rem)
		i++
	} else {
		leftStart := len(pay.leftover) - (s255s * maxSegSize) - rem
		good := payload{pay.leftover[0:leftStart], nil, nil}
		bad := payload{pay.leftover[leftStart:], pay.packets, nil}
		return segtbl, good, bad
	}

	// Now loop through the rest and track if we need to split
	for p := 0; p < len(pay.packets); p++ {
		s255s := len(pay.packets[p]) / maxSegSize
		rem := len(pay.packets[p]) % maxSegSize
		for i < len(segtbl) && s255s > 0 {
			segtbl[i] = maxSegSize
			i++
			s255s--
		}
		if i < maxSegSize {
			segtbl[i] = byte(rem)
			i++
		} else {
			right := len(pay.packets[p]) - (s255s * maxSegSize) - rem
			good := payload{pay.leftover, pay.packets[0:p], pay.packets[p][0:right]}
			bad := payload{pay.packets[p][right:], pay.packets[p+1:], nil}
			return segtbl, good, bad
		}
	}

	good := pay
	bad := payload{}
	return segtbl[0:i], good, bad
}
