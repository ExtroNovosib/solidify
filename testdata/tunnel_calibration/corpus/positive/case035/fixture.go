package fixture

import "example.com/tunnelcalibration/dep"

func NewChangeMemberRole(auth *dep.Service) *dep.Service {
	if auth == nil {
		return nil
	}
	return auth
}
