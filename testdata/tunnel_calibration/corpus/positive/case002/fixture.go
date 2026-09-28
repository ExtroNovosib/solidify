package fixture

import (
	"bytes"
	"io"
)

type Stream struct {
	base    io.Reader
	pending []byte
}

func decode(b []byte) ([]byte, error) { return b, nil }
func (c *Stream) Read(p []byte) (int, error) {
	b := make([]byte, 6)
	_, err := io.ReadFull(c.base, b)
	if err != nil {
		return 0, err
	}
	pt, err := decode(b)
	if err != nil {
		return 0, err
	}
	n := copy(p, pt)
	if n < len(pt) {
		return n, io.ErrShortBuffer
	}
	return n, nil
}
func Wrap() io.Reader { return &Stream{base: bytes.NewReader([]byte("abcdef"))} }
