package cli

import "github.com/ExtroNovosib/solidify/internal/analyzer"

// checkExample is the compact before/after pair and legitimate exception that
// `checks explain` shows for one check. Where a docs/checks page carries a
// concrete example, the pair mirrors it.
type checkExample struct {
	before, after, exception string
}

// checkExamples has one entry per registered check;
// TestChecksExplainExamplesAreCheckSpecific enforces completeness and that
// no two checks share an example.
var checkExamples = map[analyzer.CheckID]checkExample{
	analyzer.CheckLSPDiscardedRead:     {before: "Read consumes a frame, copies a prefix and returns io.ErrShortBuffer without retaining the suffix.", after: "Cache the unread suffix and drain it before consuming another frame.", exception: "An explicit record API that is not exposed as a standard byte stream."},
	analyzer.CheckLSPNoopDeadline:      {before: "A net.Conn SetReadDeadline ignores time.Time and returns nil.", after: "Delegate to a deadline-capable connection or apply an equivalent deadline.", exception: "An unrelated setter on a type that does not implement net.Conn."},
	analyzer.CheckISPConstructorRole:   {before: "NewRunner accepts WideStore but stores a narrow executionStore.", after: "Accept the stored consumer interface, including variadic elements.", exception: "The constructor actually consumes extra broad-interface methods or the value escapes."},
	analyzer.CheckSRPTransportWorkflow: {before: "An HTTP handler constructs domain aggregates then creates and saves them directly.", after: "Inject an application command that owns the ordered workflow.", exception: "Decode, invoke an injected command, and encode its result; single reads alone."},
	analyzer.CheckSRPGodType: {
		before:    "type OrderManager struct { db *sql.DB; mail *smtp.Client; cache *redis.Client }; func (m *OrderManager) Checkout() {}; func (m *OrderManager) EmailReceipt() {}; func (m *OrderManager) SalesReport() {}",
		after:     "type Checkout struct { orders OrderStore }; type Receipts struct { mailer Mailer }; type SalesReport struct { cache ReportCache }",
		exception: "A facade whose methods each delegate to one cohesive collaborator can look complex while every call stays thin orchestration.",
	},
	analyzer.CheckSRPLowCohesionType: {
		before:    "type Account struct { balance int; email string; theme string }; func (a *Account) Deposit(n int) { a.balance += n }; func (a *Account) Withdraw(n int) { a.balance -= n }; func (a *Account) Notify() { send(a.email) }; func (a *Account) Restyle() { a.theme = \"dark\" }",
		after:     "type Ledger struct { balance int }; type Contact struct { email string }; type Preferences struct { theme string }",
		exception: "Handlers and other types named by srp.orchestrator_suffixes intentionally coordinate unrelated steps, and serialized data carriers are skipped.",
	},
	analyzer.CheckSRPLargeType: {
		before:    "type LargeService struct { ID int; Name string; Enabled bool; Payload []byte; Metadata map[string]string /* eleven heterogeneous fields */ }; func (*LargeService) Create() {} /* eleven exported methods, WMC >= 47 */",
		after:     "type Catalog struct { items map[string]Item }; type Publisher struct { queue Queue }; type Auditor struct { log AuditLog }",
		exception: "Generated models, DTOs, and framework-owned structures can be intentionally broad when an external schema owns their shape.",
	},
	analyzer.CheckSRPHighFanOutType: {
		before:    "type Importer struct { csv *csv.Reader; zip *zip.Reader; s3 *s3.Client; db *sql.DB; tpl *template.Template /* sixteen foreign types in fields and signatures */ }",
		after:     "type Importer struct { source RecordSource; sink RecordSink }",
		exception: "An adapter that translates between many external SDK types may legitimately reference each of them.",
	},
	analyzer.CheckSRPComplexFunction: {
		before:    "func Process(order Order) error { /* 70 lines and 16 branches that validate, price, persist, and notify */ }",
		after:     "func Process(order Order) error { if err := validate(order); err != nil { return err }; return persist(price(order)) }",
		exception: "Table-driven parsers and generated state machines can be long and branchy while remaining one responsibility.",
	},
	analyzer.CheckSRPMixedInputSurface: {
		before:    "func Coordinate(ctx context.Context, request Request, store Store, logger Logger, retries int, notify func(), clock Clock, limits Limits, region string) {}",
		after:     "type Coordinator struct { store Store; logger Logger; clock Clock }; func (c *Coordinator) Run(ctx context.Context, job Job) {}",
		exception: "Homogeneous inputs such as coordinates or batch values are cohesive and are not reported.",
	},
	analyzer.CheckSRPDataClump: {
		before:    "func RegisterContact(name, email, phone, address, city, country, region, postal, company string) {}; func UpdateContact(name, email, phone, address, city, country, region, postal, company string) {}",
		after:     "type Contact struct { Name, Email, Phone, Address, City, Country, Region, Postal, Company string }; func RegisterContact(contact Contact) {}; func UpdateContact(contact Contact) {}",
		exception: "A stable exported API may keep a repeated parameter list for compatibility until its next major version.",
	},
	analyzer.CheckSRPFlagArgument: {
		before:    "func Render(document string, compact bool) string { if compact { return shrink(document) } else { return document } }",
		after:     "func Render(document string) string { return document }; func RenderCompact(document string) string { return shrink(document) }",
		exception: "A boolean that only toggles a minor presentation detail, without separate behavior paths, can remain a parameter.",
	},
	analyzer.CheckSRPMixedImportClusters: {
		before:    "func (s *Service) Export(w io.Writer) { csv.NewWriter(w); zip.NewWriter(w) }; func (s *Service) Charge() { stripe.Charge(); ledger.Post() }; func (s *Service) Notify() { smtp.SendMail(); template.New(\"mail\") }",
		after:     "type Exporter struct{}; type Billing struct{}; type Notifier struct{}",
		exception: "A facade that deliberately spans subsystems can import several clusters while each method delegates to one collaborator.",
	},
	analyzer.CheckOCPTypeDispatch: {
		before:    "func Dispatch(value any) { switch value.(type) { case A, B, C, D, E: } }",
		after:     "type Handler interface { Handle() }; func Dispatch(value Handler) { value.Handle() }",
		exception: "A closed protocol decoder or AST visitor may intentionally enumerate a finite set; go/, encoding/, and reflect sources are allowed by default.",
	},
	analyzer.CheckOCPDiscriminatorDispatch: {
		before:    "func Price(o Order) int { switch o.Kind { case Retail: return 1; case Wholesale: return 2; case Partner: return 3 }; return 0 }; func Ship(o Order) { switch o.Kind { case Retail, Wholesale, Partner: } }",
		after:     "type Channel interface { Price(Order) int; Ship(Order) }; var channels = map[Kind]Channel{Retail: retail{}, Wholesale: wholesale{}, Partner: partner{}}",
		exception: "A serialization boundary that maps a wire enum onto domain types may switch on the discriminator once per direction.",
	},
	analyzer.CheckOCPRuntimeExhaustiveness: {
		before:    "switch shape := s.(type) { case Circle: return shape.Area(); case Square: return shape.Area(); default: panic(fmt.Sprintf(\"unknown shape %T\", s)) }",
		after:     "type Shape interface { Area() float64 }; func Area(s Shape) float64 { return s.Area() }",
		exception: "A decoder validating untrusted input may reject unknown variants at runtime by design; prefer returning an error to panicking.",
	},
	analyzer.CheckOCPConcreteParameter: {
		before:    "func ProcessOrder(o *orders.Order) float64 { return o.Validate() + o.Total() }",
		after:     "type Totaler interface { Validate() float64; Total() float64 }; func ProcessOrder(o Totaler) float64 { return o.Validate() + o.Total() }",
		exception: "A parameter that must stay one concrete type for identity or performance can be listed in allow_dependencies.",
	},
	analyzer.CheckOCPClosedFactory: {
		before:    "func MakeShape(kind string) Shape { switch kind { case \"circle\": return Circle{}; case \"square\": return Square{}; case \"triangle\": return Triangle{} }; return nil }",
		after:     "var shapes = map[string]func() Shape{}; func Register(kind string, build func() Shape) { shapes[kind] = build }; func MakeShape(kind string) Shape { return shapes[kind]() }",
		exception: "A composition root or a small enumeration owned by one package may intentionally construct every variant.",
	},
	analyzer.CheckOCPImplementationCoupling: {
		before:    "package checkout; import (\"example.com/app/providers/paypal\"; \"example.com/app/providers/stripe\"); type Service struct { stripe stripe.Client; paypal paypal.Client }",
		after:     "package checkout; type PaymentGateway interface { Charge(amount int) error }; type Service struct { gateway PaymentGateway }",
		exception: "Packages listed in architecture.composition_roots wire implementations and are excluded by design.",
	},
	analyzer.CheckOCPParallelImplementations: {
		before:    "func ProcessCircle(v *Circle) float64 { total := v.Area(); if total > 0 { total += v.Perimeter() }; return total }; func ProcessSquare(v *Square) float64 { /* same body */ }; func ProcessTriangle(v *Triangle) float64 { /* same body */ }",
		after:     "type Shape interface { Area() float64; Perimeter() float64 }; func Process(v Shape) float64 { total := v.Area(); if total > 0 { total += v.Perimeter() }; return total }",
		exception: "Generated or performance-specialized code may duplicate an algorithm per concrete type on purpose.",
	},
	analyzer.CheckLSPNonExactEOF: {
		before:    "return 0, fmt.Errorf(\"EOF: %w\", io.EOF)",
		after:     "return 0, io.EOF",
		exception: "An adapter may normalize behavior only when its public contract explicitly documents the changed substitution semantics.",
	},
	analyzer.CheckLSPNilEmbeddedInterface: {
		before:    "type ReadOnlyStore struct { Store }; func NewReadOnlyStore() *ReadOnlyStore { return &ReadOnlyStore{} } // promoted Store methods panic",
		after:     "func NewReadOnlyStore(store Store) *ReadOnlyStore { return &ReadOnlyStore{Store: store} }",
		exception: "A test double that embeds an interface only to satisfy it, and never calls the promoted methods, can leave the embedding nil.",
	},
	analyzer.CheckISPFatInterface: {
		before:    "type WidePort interface { A(); B(); C(); D(); E(); F(); G(); H(); I() }",
		after:     "type Reader interface { A(); B() }; type Writer interface { C(); D() }; func load(port Reader) { port.A() }",
		exception: "A framework facade or migration adapter may deliberately aggregate capabilities after its actual consumers are reviewed.",
	},
	analyzer.CheckISPUsageRatio: {
		before:    "type Repository interface { Get() error; Save() error; Delete() error }; func UseRepository(repository Repository) error { return repository.Get() }",
		after:     "type Getter interface { Get() error }; func UseRepository(repository Getter) error { return repository.Get() }",
		exception: "Standard wide interfaces such as context.Context and http.ResponseWriter are excluded; a deliberately broad port may be accepted.",
	},
	analyzer.CheckISPConsumerRole: {
		before:    "type TraceStore interface { ListStages() error; ListChunks() error; GetChunks() error; ListExtractions() error; StartStage() error; FinishStage() error }; type TraceService struct { trace TraceStore } // calls only the four queries",
		after:     "type TraceReader interface { ListStages() error; ListChunks() error; GetChunks() error; ListExtractions() error }; type TraceService struct { trace TraceReader }",
		exception: "A consumer planned to grow into the full role soon may keep the broader interface once that roadmap is reviewed.",
	},
	analyzer.CheckISPUnusedDependency: {
		before:    "type Ports struct { Unused Store } // no consumer ever selects or forwards Ports.Unused",
		after:     "type Ports struct{} // remove the field, or wire it into the collaborator that calls it",
		exception: "Composition roots and wiring aggregates named by isp.wiring_aggregate_suffixes (Bundle, Deps, Dependencies, Stores) are skipped.",
	},
	analyzer.CheckISPStubImplementation: {
		before:    "type Repository interface { Get() error; Save() error }; type ReadOnlyRepository struct{}; func (ReadOnlyRepository) Get() error { return nil }; func (ReadOnlyRepository) Save() error { return errors.ErrUnsupported }",
		after:     "type Getter interface { Get() error }; type ReadOnlyRepository struct{}; func (ReadOnlyRepository) Get() error { return nil }",
		exception: "An optional capability discovered with a type assertion, such as io.WriterTo, may return ErrUnsupported by contract.",
	},
	analyzer.CheckDIPConcreteDependency: {
		before:    "func NewArchiveService(client *database.PostgreSQLClient) *ArchiveService { _ = client; return &ArchiveService{} }",
		after:     "type ArchiveStore interface { Save(Record) error }; func NewArchiveService(store ArchiveStore) *ArchiveService { return &ArchiveService{store: store} }",
		exception: "A declared composition root must wire concrete implementations and can intentionally depend on infrastructure.",
	},
	analyzer.CheckDIPLayerImport: {
		before:    "package billing // a logic package; import \"database/sql\"; func Charge(db *sql.DB) error { _, err := db.Exec(\"INSERT ...\"); return err }",
		after:     "package billing; type Ledger interface { Record(Charge) error }; func Charge(ledger Ledger, c Charge) error { return ledger.Record(c) }",
		exception: "A logic package that owns its persistence by design can drop the import from dip.detail_imports or leave architecture.logic_packages.",
	},
	analyzer.CheckDIPWiringOutsideRoot: {
		before:    "package handlers; func Routes() http.Handler { svc := billing.NewService(postgres.NewStore(dsn)); return api.New(svc) }",
		after:     "package main; func main() { svc := billing.NewService(postgres.NewStore(dsn)); _ = http.ListenAndServe(addr, api.New(svc)) }",
		exception: "Tests and command entry points listed in architecture.composition_roots may construct implementations directly.",
	},
	analyzer.CheckDIPHiddenConstruction: {
		before:    "func NewService() *Service { return &Service{store: postgres.NewStore()} }",
		after:     "func NewService(store Store) *Service { return &Service{store: store} }",
		exception: "A constructor may build a private default for an optional collaborator when an option lets callers override it.",
	},
	analyzer.CheckDIPInfraErrorLeak: {
		before:    "package accounts; func Find(repo Repo, id string) error { if err := repo.Get(id); errors.Is(err, sql.ErrNoRows) { return ErrNotFound }; return nil }",
		after:     "package accounts; var ErrNotFound = errors.New(\"account not found\"); func Find(repo Repo, id string) error { return repo.Get(id) } // the adapter translates sql.ErrNoRows",
		exception: "An adapter package that owns the database boundary should translate infrastructure errors; keep it out of architecture.logic_packages.",
	},
	analyzer.CheckDIPTransportLeak: {
		before:    "package accounts; func Register(r *http.Request) error { return save(r.FormValue(\"email\")) }",
		after:     "package accounts; type Registration struct { Email string }; func Register(input Registration) error { return save(input.Email) } // the handler decodes the request",
		exception: "Middleware and handlers in transport packages naturally use *http.Request; only logic-package signatures are checked.",
	},
}
