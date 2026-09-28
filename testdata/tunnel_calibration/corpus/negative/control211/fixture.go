package fixture

type Port interface{ Apply() }
type Carrier struct {
	Transport Port
	Unused    Port
}
type Consumer struct{ ports Carrier }

func (c *Consumer) Handle() { c.ports.Transport.Apply() }
