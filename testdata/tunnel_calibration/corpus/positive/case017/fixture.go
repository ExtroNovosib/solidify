package fixture

type Store interface {
	Get()
	Count()
	Create()
	Delete()
	List()
	Save()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.Get() }
