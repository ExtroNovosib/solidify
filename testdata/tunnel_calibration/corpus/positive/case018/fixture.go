package fixture

type Store interface {
	ActiveEventGrant()
	CountActiveByOwner()
	Create()
	Delete()
	Get()
	GetByHash()
	ListByEndpoint()
	ListByOwner()
	Save()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ActiveEventGrant() }
