package fixture

import "example.com/tunnelcalibration/dep"

func NewEvaluatorWorker(due *dep.Service) *dep.Service {
	if due == nil {
		return nil
	}
	return due
}
