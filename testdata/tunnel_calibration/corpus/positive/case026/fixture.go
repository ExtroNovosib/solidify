package fixture

type Store interface {
	ClaimDue()
	CompleteTask()
	CreateTask()
	GetTask()
	ListTasksByRun()
	MarkCancelRequested()
	RenewLease()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ClaimDue() }
