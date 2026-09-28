package fixture

type Store interface {
	FindByHash()
	TouchLastUsed()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.FindByHash(); c.store.TouchLastUsed() }
