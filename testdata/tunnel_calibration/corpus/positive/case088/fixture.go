package fixture

type Port interface{ Apply() }
type Consumer struct{ Limits Port }

func (c *Consumer) Handle() {}
