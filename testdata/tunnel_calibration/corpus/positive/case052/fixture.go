package fixture

import "example.com/tunnelcalibration/dep"

func NewWorker(exec *dep.Service) *dep.Service {
	if exec == nil {
		return nil
	}
	return exec
}
