package fixture

import "example.com/tunnelcalibration/dep"

type DetachEndpoint struct{ Auth *dep.Service }

func (c *DetachEndpoint) Handle() error {
	if c.Auth == nil {
		return nil
	}
	return c.Auth.Handle()
}
