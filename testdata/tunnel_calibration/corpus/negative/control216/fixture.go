package fixture

type Port interface{ Apply() }
type Carrier struct {
	Writer Port
	Unused Port
}
type Consumer struct{ ports Carrier }

func (c *Consumer) Handle() { c.ports.Writer.Apply() }
