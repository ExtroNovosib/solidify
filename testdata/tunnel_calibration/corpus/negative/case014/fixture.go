package fixture

type Store interface{ ListJobs() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ListJobs() }
