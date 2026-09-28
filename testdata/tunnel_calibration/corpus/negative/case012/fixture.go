package fixture

type Store interface{ ListByOwner() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ListByOwner() }
