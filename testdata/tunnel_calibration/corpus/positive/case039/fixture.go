package fixture

import "example.com/tunnelcalibration/dep"

func NewDeleteEventNote(auth *dep.Service) *dep.Service {
	if auth == nil {
		return nil
	}
	return auth
}
