package fixture

import (
	"net"
	"time"
)

type Stream struct{ net.Conn }

func (c *Stream) SetReadDeadline(t time.Time) error { return c.Conn.SetReadDeadline(t) }
