package fixture

type Port interface{ Apply() }
type Consumer struct{ Clock Port }

func (c *Consumer) Handle() {}
