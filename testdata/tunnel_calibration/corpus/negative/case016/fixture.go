package fixture

type Store interface{ List() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.List() }
