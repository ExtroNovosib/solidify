package violations

import (
	"errors"
	"fmt"
)

// ---- SRP violation: a god object with too many methods ----

type UserService struct {
	count    int
	name     string
	enabled  bool
	ratio    float64
	code     byte
	symbol   rune
	phase    complex64
	mask     uint
	address  uintptr
	payload  []byte
	metadata map[string]string
}

func (s *UserService) CreateUser()       {}
func (s *UserService) DeleteUser()       {}
func (s *UserService) UpdateUser()       {}
func (s *UserService) SendEmail()        {}
func (s *UserService) SendSMS()          {}
func (s *UserService) GeneratePDF()      {}
func (s *UserService) LogAudit()         {}
func (s *UserService) ValidateInput()    {}
func (s *UserService) HashPassword()     {}
func (s *UserService) ChargeCreditCard() {}
func (s *UserService) ExportToCSV()      {}
func (s *UserService) ImportFromCSV()    {}
func (s *UserService) PublishWebhook()   {}
func (s *UserService) RevokeSession()    {}
func (s *UserService) AssignRole()       {}
func (s *UserService) RemoveRole()       {}
func (s *UserService) SyncCRM()          {}
func (s *UserService) ArchiveAccount()   {}
func (s *UserService) RestoreAccount()   {}
func (s *UserService) GenerateInvoice()  {}
func (s *UserService) RecordAnalytics()  {}

// SRP violation: the same contact-data clump crosses multiple functions.
func Register(name, email, phone, address, city, country, region, postal, company string) error {
	return nil
}

func UpdateContact(name, email, phone, address, city, country, region, postal, company string) error {
	return nil
}

// ---- OCP violation: type switch that must grow with every new shape ----

type Shape interface{ Kind() string }
type Circle struct{ R float64 }
type Square struct{ Side float64 }
type Triangle struct{ Base, H float64 }
type Rect struct{ W, H float64 }
type Pentagon struct{ Side float64 }

func (Circle) Kind() string   { return "circle" }
func (Square) Kind() string   { return "square" }
func (Triangle) Kind() string { return "triangle" }
func (Rect) Kind() string     { return "rect" }
func (Pentagon) Kind() string { return "pentagon" }

func Area(s Shape) float64 {
	switch v := s.(type) {
	case Circle:
		return 3.14159 * v.R * v.R
	case Square:
		return v.Side * v.Side
	case Triangle:
		return 0.5 * v.Base * v.H
	case Rect:
		return v.W * v.H
	case Pentagon:
		return 1.72 * v.Side * v.Side
	default:
		return 0
	}
}

// ---- ISP violation: implementation forced to reject part of the interface contract ----

type Repository interface {
	Get(id string) (string, error)
	Save(v string) error
	Delete(id string) error
}

type ReadOnlyRepository struct{}

func (r *ReadOnlyRepository) Get(id string) (string, error) {
	return "", nil
}

func (r *ReadOnlyRepository) Save(v string) error {
	panic("save is not supported on a read-only repository")
}

func (r *ReadOnlyRepository) Delete(id string) error {
	return errors.ErrUnsupported
}

func UseRepository(r Repository) error {
	return r.Save("x")
}

// ---- ISP violation: a fat interface ----

type Machine interface {
	Print(doc string)
	Scan() string
	Fax(doc string)
	Copy() string
	Staple()
	CollateAndBind()
	Shred()
	Laminate()
	Bind()
}

// ---- DIP violation: high-level type wired directly to a concrete type ----

type SmtpMailer struct{}

func (m *SmtpMailer) Send(to, body string) error {
	fmt.Println("sending", to, body)
	return nil
}

type OrderProcessor struct {
	mailer *SmtpMailer // should depend on an interface instead
}
