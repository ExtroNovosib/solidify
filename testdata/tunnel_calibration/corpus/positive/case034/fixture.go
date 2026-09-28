package fixture

import "example.com/tunnelcalibration/dep"

type ChangeMemberRole struct{ Auth *dep.Service }

func (c *ChangeMemberRole) Handle() error {
	if c.Auth == nil {
		return nil
	}
	return c.Auth.Handle()
}
