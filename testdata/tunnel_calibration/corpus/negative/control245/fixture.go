package fixture

type Port interface {
	Get()
	List()
}
type Consumer struct{ store Port }

func NewConsumer(p Port) *Consumer { return &Consumer{p} }
func (c *Consumer) Handle()        { c.store.Get(); c.store.List() }
