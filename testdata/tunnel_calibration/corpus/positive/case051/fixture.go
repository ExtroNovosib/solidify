package fixture

import "example.com/tunnelcalibration/dep"

type Worker struct{ Executor *dep.Service }

func (c *Worker) Handle() error {
	if c.Executor == nil {
		return nil
	}
	return c.Executor.Handle()
}
