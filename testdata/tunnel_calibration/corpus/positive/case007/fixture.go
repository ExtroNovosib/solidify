package fixture

import "example.com/tunnelcalibration/dep"

type SessionResolver struct{ AuthSrv *dep.Service }

func (c *SessionResolver) Handle() error {
	if c.AuthSrv == nil {
		return nil
	}
	return c.AuthSrv.Handle()
}
