package fixture

type Broad interface {
	ClaimDue()
	CompleteTask()
	CreateTask()
	GetTask()
	ListTasksByRun()
	MarkCancelRequested()
	RenewLease()
}
type Narrow interface{ CompleteTask() }
type Runner struct{ store Narrow }

func NewRunner(taskStore Broad) *Runner { return &Runner{taskStore} }
