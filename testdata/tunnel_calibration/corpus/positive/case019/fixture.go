package fixture

type Store interface {
	CountByEvent()
	Create()
	Delete()
	Get()
	ListByEvent()
	Save()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.CountByEvent(); c.store.Create() }
