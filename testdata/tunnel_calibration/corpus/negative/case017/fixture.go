package fixture

type Store interface{ Get() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.Get() }
