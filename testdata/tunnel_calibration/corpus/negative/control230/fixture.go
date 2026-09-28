package fixture

type Role0 interface {
	M0()
	M1()
	M2()
	M3()
}
type Role1 interface {
	M4()
	M5()
	M6()
	M7()
}
type Role2 interface {
	M8()
	M9()
	M10()
	M11()
}
type Role3 interface {
	M12()
	M13()
	M14()
}
type Store interface {
	Role0
	Role1
	Role2
	Role3
}
type Service struct{ store Store }

func (s *Service) Handle() {
	s.store.M0()
	s.store.M1()
	s.store.M2()
	s.store.M3()
	s.store.M4()
	s.store.M5()
	s.store.M6()
	s.store.M7()
	s.store.M8()
	s.store.M9()
	s.store.M10()
	s.store.M11()
	s.store.M12()
	s.store.M13()
	s.store.M14()
}
