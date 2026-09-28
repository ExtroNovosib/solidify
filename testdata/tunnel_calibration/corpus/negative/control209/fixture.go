package fixture

type Port interface{ Apply() }
type Carrier struct {
	WebSocket Port
	Unused    Port
}
type Consumer struct{ ports Carrier }

func (c *Consumer) Handle() { c.ports.WebSocket.Apply() }
