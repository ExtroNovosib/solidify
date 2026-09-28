package fixture

type Port interface{ Handle() error }
type Consumer struct{ dep Port }

func NewConsumer(p Port) *Consumer { return &Consumer{p} }
func (c *Consumer) Handle() error {
	if c.dep == nil {
		return nil
	}
	return c.dep.Handle()
}
