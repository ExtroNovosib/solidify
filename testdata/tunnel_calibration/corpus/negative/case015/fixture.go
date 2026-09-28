package fixture

type Store interface{ Counts() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.Counts() }
