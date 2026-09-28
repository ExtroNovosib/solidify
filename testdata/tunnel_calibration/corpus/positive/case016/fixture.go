package fixture

type Store interface {
	List()
	Counts()
	MostRecent()
	TunnelTotals()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.List() }
