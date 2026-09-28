package fixture

type Store interface {
	FindByID()
	Create()
	FindByLogin()
	UpdatePassword()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.FindByID() }
