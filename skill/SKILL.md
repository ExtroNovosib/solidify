---
name: go-solid
description: Write idiomatic Go that follows SOLID (SRP, OCP, LSP, ISP, DIP). Use when writing, generating, refactoring, or reviewing Go code; when designing packages, interfaces, constructors, or DI wiring; or when the user mentions SOLID, solidlint, or Go architecture.
user-invocable: true
disable-model-invocation: false
---

# Go SOLID Code

Write Go that stays SOLID without Java ceremony. Prefer small packages, consumer-owned interfaces, and composition roots for wiring.

## When this applies

Apply on every Go write/refactor/review unless user asks otherwise.

## Defaults (Go-shaped SOLID)

1. **Accept interfaces, return structs** — define interfaces at the consumer, not the producer.
2. **Small interfaces** — prefer 1–3 methods; split before stubs/`ErrUnsupported`/panic.
3. **Depend on behavior, not concrete cross-package types** — inject ports; construct concretes only in composition roots (`main`, `cmd`, `internal/app`).
4. **Extend via new types** — avoid growing type-switches / discriminator branches for every variant.
5. **One reason to change per type** — split god services; keep DTOs passive.
6. **No fake LSP** — do not embed interfaces left nil; honor contracts exactly (e.g. `io.EOF` identity).

## Principle playbook

### S — Single Responsibility

| Do | Don't |
|----|-------|
| One cohesive job per type/package | God types mixing auth, billing, IO, PDF, CRM |
| Extract helpers when a function is hard to name | Flag args that branch unrelated behaviors (`process(x, isAdmin bool)`) |
| Group repeated param clumps into a struct | Same 8+ field contact blob across many funcs |
| Keep import clusters coherent with the type's job | One type importing DB + HTTP + mail + metrics for unrelated methods |

**Split signals:** many unrelated methods, mixed import clusters, low cohesion, huge functions, mixed input surfaces.

### O — Open/Closed

| Do | Don't |
|----|-------|
| Behavior on the type (`Area() float64`) or registry/strategy | Large/repeated `switch t.(type)` / Kind/Type/Status chains that grow per variant |
| Pass interfaces when only methods are used | Concrete params used only as method bags |
| Open factories (inject / register impls) | Closed `NewX(kind string)` switches over all impls |
| Share algorithm, vary strategy | Copy-paste parallel impls that differ only in literals |

One-off small switches OK. Repeated/large dispatch across the module is the smell.

### L — Liskov Substitution

| Do | Don't |
|----|-------|
| Implementations that honor the full contract | Stub methods (`panic`, `ErrUnsupported`) for unused interface parts → shrink interface (ISP) |
| Return real `io.EOF` when that is the contract | Wrap/rebuild EOF so `errors.Is`/`==` break |
| Initialize every embedded interface | Embed `io.Reader` (etc.) and leave it nil |

### I — Interface Segregation

| Do | Don't |
|----|-------|
| Role interfaces (`Reader`, `Saver`) composed if needed | Fat kitchen-sink interfaces |
| Interfaces matching actual call sites | Clients forced to depend on unused methods |
| Consumer-defined ports | Producer dumping a 10-method "service API" |

If impl must no-op/stub a method → interface too wide.

### D — Dependency Inversion

| Do | Don't |
|----|-------|
| Logic depends on interfaces | Logic fields/`New*` take `*PostgresRepo`, `*SmtpMailer` from another package |
| Wire concretes in composition roots | Hidden `NewFoo()` constructing impls inside factories returned as deps |
| Map infra errors at the adapter boundary | Logic compares `sql.ErrNoRows` / leaks `*http.Request`, `*sql.Tx` |
| Logic packages import ports, not adapters | `internal/service` imports `internal/adapters/postgres` |

Same-package concretes and `*Config` bags are fine. Stdlib concretes often OK. Cross-package behavior deps → interface.

## Package layout sketch

```text
cmd/app/            # composition root: build concretes, inject
internal/app/       # optional wiring
internal/domain/    # pure types, ports (interfaces) as needed
internal/service/   # use-cases; depend on ports only
internal/adapters/  # postgres, http, smtp — implement ports
```

## Code patterns

**DIP + ISP — consumer port**

```go
type Notifier interface {
	Notify(msg string) error
}

type Order struct {
	notifier Notifier
}

func NewOrder(n Notifier) *Order {
	return &Order{notifier: n}
}
```

**OCP — behavior on type, not type switch**

```go
type Shape interface{ Area() float64 }

func (c Circle) Area() float64 { return math.Pi * c.R * c.R }
```

**Composition root only**

```go
func main() {
	mailer := smtp.NewMailer(cfg)
	orders := service.NewOrder(mailer) // mailer satisfies Notifier
	http.ListenAndServe(addr, api.NewServer(orders))
}
```

## Workflow when writing Go

1. Name the responsibility of each new type/package in one phrase.
2. Put cross-boundary deps behind small consumer interfaces.
3. Construct concretes only in `main`/`cmd`/`app`.
4. Prefer methods/strategies over growing type switches.
5. If an impl cannot honor a method → split the interface.
6. Keep transport/DB types out of logic signatures.
7. When `solidlint` / this repo available, scan package dirs (`./internal/foo/`), not single files.

## Anti-overengineering

- Do **not** interface every struct. Interface at **boundaries** and where multiple impls or tests need fakes.
- Do **not** add DI frameworks by default. Func constructors + interfaces enough.
- DTOs with no behavior are not SRP violations.
- Homogeneous CRUD on one aggregate can stay one type if cohesion is real.

## Review output (when reviewing)

For each issue: principle (S/O/L/I/D), location, smell, minimal fix. Prefer split/extract/inject over suppress comments.
