package fixture

type Store interface{ FindByID() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.FindByID() }
