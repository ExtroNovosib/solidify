package fixture

import "example.com/tunnelcalibration/dep"

func NewEvaluateSignal(evaluators *dep.Service) *dep.Service {
	if evaluators == nil {
		return nil
	}
	return evaluators
}
