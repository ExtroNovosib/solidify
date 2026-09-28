package fixture

type Store interface {
	ListByOwner()
	CountActiveByOwner()
	Create()
	GetByHash()
	MarkUsed()
	Revoke()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ListByOwner() }
