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
	if len(p) == 0 {
		return 0, nil
	}
	if len(c.pending) > 0 {
		n := copy(p, c.pending)
		c.pending = c.pending[n:]
		return n, nil
	}
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
	c.pending = pt[n:]
	if n < len(pt) {
		return n, nil
	}
	return n, nil
}
func Wrap() io.Reader { return &Stream{base: bytes.NewReader([]byte("abcdef"))} }
