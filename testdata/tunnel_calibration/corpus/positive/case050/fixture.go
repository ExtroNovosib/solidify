package fixture

import "example.com/tunnelcalibration/dep"

type Worker struct{ Runner *dep.Service }

func (c *Worker) Handle() error {
	if c.Runner == nil {
		return nil
	}
	return c.Runner.Handle()
}
