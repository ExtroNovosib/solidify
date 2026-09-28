package fixture

type Store interface {
	ClaimItems()
	CountActiveJobs()
	CreateJob()
	CreateJobIfBelowLimit()
	GetAttemptByExecution()
	GetItem()
	GetJob()
	InsertAttempt()
	ListJobs()
	ReserveAttemptNumber()
	SaveItem()
	SaveJob()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ClaimItems() }
