package fixture

type Store interface{ GetJob() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.GetJob() }
