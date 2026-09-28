package fixture

type Port interface{ Apply() }
type Consumer struct{}

func (c *Consumer) Handle() {}
