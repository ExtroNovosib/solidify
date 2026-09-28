package fixture

type Port interface{ Handle(string) string }
type Application struct{ A, B, C Port }

func (a *Application) M0(v string) string {
	if a == nil || a.A == nil {
		return ""
	}
	return a.A.Handle(v)
}
func (a *Application) M1(v string) string {
	if a == nil || a.B == nil {
		return ""
	}
	return a.B.Handle(v)
}
func (a *Application) M2(v string) string {
	if a == nil || a.C == nil {
		return ""
	}
	return a.C.Handle(v)
}
func (a *Application) M3(v string) string {
	if a == nil || a.A == nil {
		return ""
	}
	return a.A.Handle(v)
}
func (a *Application) M4(v string) string {
	if a == nil || a.B == nil {
		return ""
	}
	return a.B.Handle(v)
}
func (a *Application) M5(v string) string {
	if a == nil || a.C == nil {
		return ""
	}
	return a.C.Handle(v)
}
func (a *Application) M6(v string) string {
	if a == nil || a.A == nil {
		return ""
	}
	return a.A.Handle(v)
}
func (a *Application) M7(v string) string {
	if a == nil || a.B == nil {
		return ""
	}
	return a.B.Handle(v)
}
func (a *Application) M8(v string) string {
	if a == nil || a.C == nil {
		return ""
	}
	return a.C.Handle(v)
}
func (a *Application) M9(v string) string {
	if a == nil || a.A == nil {
		return ""
	}
	return a.A.Handle(v)
}
func (a *Application) M10(v string) string {
	if a == nil || a.B == nil {
		return ""
	}
	return a.B.Handle(v)
}
func (a *Application) M11(v string) string {
	if a == nil || a.C == nil {
		return ""
	}
	return a.C.Handle(v)
}
