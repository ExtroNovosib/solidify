package fixture

import "example.com/tunnelcalibration/dep"

type RemoveMember struct{ Auth *dep.Service }

func (c *RemoveMember) Handle() error {
	if c.Auth == nil {
		return nil
	}
	return c.Auth.Handle()
}
