package fixture

type Port interface{ Apply() }
type Carrier struct {
	Lister Port
	Unused Port
}
type Consumer struct{ ports Carrier }

func (c *Consumer) Handle() { c.ports.Lister.Apply() }
