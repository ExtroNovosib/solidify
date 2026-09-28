package fixture

import (
	"net"
	"time"
)

type Stream struct{ net.Conn }

func (c *Stream) SetWriteDeadline(t time.Time) error { return c.Conn.SetWriteDeadline(t) }
