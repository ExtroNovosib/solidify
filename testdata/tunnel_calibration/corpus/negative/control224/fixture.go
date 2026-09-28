package fixture

type Port interface{ Apply() }
type Carrier struct {
	Repos  Port
	Unused Port
}
type Consumer struct{ ports Carrier }

func (c *Consumer) Handle() { c.ports.Repos.Apply() }
