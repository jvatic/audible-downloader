package audiometa

import (
	"io"
	"sync"
)

func parseHeader(rd io.Reader) (int64, error) {
	data := make([]byte, 10)
	_, err := rd.Read(data)
	if err != nil {
		return 0, err
	}

	// Tag header size is always synchsafe.
	size, err := parseSize(data[6:])
	if err != nil {
		return 0, err
	}
	return size + 10, nil
}

func parseSize(data []byte) (int64, error) {
	var sizeBase uint = 7
	var size int64
	for _, b := range data {
		size = (size << sizeBase) | int64(b)
	}

	return size, nil
}

var bsPool = sync.Pool{
	New: func() interface{} { return nil },
}

// getByteSlice returns []byte with len == size.
func getByteSlice(size int) []byte {
	fromPool := bsPool.Get()
	if fromPool == nil {
		return make([]byte, size)
	}
	bs := fromPool.([]byte)
	if cap(bs) < size {
		bs = make([]byte, size)
	}
	return bs[0:size]
}

// putByteSlice puts b to pool.
func putByteSlice(b []byte) {
	bsPool.Put(b)
}
