package fixture

import "example.com/tunnelcalibration/dep"

type AssignEndpoint struct{ Auth *dep.Service }

func (c *AssignEndpoint) Handle() error {
	if c.Auth == nil {
		return nil
	}
	return c.Auth.Handle()
}
