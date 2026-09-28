package analyzer

import (
	"go/ast"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func tunnelPackages(t *testing.T, sources map[string]string) []*packageFiles {
	t.Helper()
	dir := t.TempDir()
	sources["go.mod"] = "module example.com/calibration\n\ngo 1.25\n"
	for name, source := range sources {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	packages, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	return packages
}

func TestTunnelPolicyConfigAndCoverage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yml")
	if err := os.WriteFile(path, []byte("profile: all\nisp:\n  execution_methods: [TryClaimRuleUse]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := LoadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	file.Apply(&cfg)
	if !slices.Equal(cfg.ISPExecutionMethods, []string{"TryClaimRuleUse"}) {
		t.Fatalf("capability policy: %+v", cfg)
	}
	for _, typed := range []bool{false, true} {
		plan, err := NewExecutionPlan(cfg, allRulesEnabled(), SurfaceCLI)
		if err != nil {
			t.Fatal(err)
		}
		coverage := coverageByCheck(t, newRunStats(plan, cfg).snapshot([]*packageFiles{{pkgPath: "example.com/p", typeComplete: typed}}))
		want := "unavailable"
		if typed {
			want = "complete"
		}
		for _, check := range []CheckID{CheckLSPDiscardedRead, CheckLSPNoopDeadline, CheckISPConstructorRole, CheckSRPTransportWorkflow} {
			if coverage[check].Status != want {
				t.Errorf("%s coverage: %+v", check, coverage[check])
			}
		}
		if coverage[CheckDIPLayerImport].Status != "configuration-unavailable" {
			t.Fatal("architecture policy availability omitted")
		}
	}
}

func TestTunnelBehavioralDomainParameter(t *testing.T) {
	pkgs := tunnelPackages(t, map[string]string{
		"domain/domain.go": `package domain
type Aggregate struct{n int}
func(a *Aggregate)Name()string{return "name"}
func(a *Aggregate)ID()int{return a.n}
func(a *Aggregate)Execute()bool{a.n++;return true}
func(a *Aggregate)Advance()int{a.n++;return a.n}
`,
		"consumer/consumer.go": `package consumer
import "example.com/calibration/domain"
func Map(a *domain.Aggregate)(string,int){return a.Name(),a.ID()}
func Run(a *domain.Aggregate)(bool,int){return a.Execute(),a.Advance()}
`})
	issues := issuesWithCheck(Run(pkgs, tunnelAllConfig(), allRulesEnabled()), CheckOCPConcreteParameter)
	if len(issues) != 1 || !strings.Contains(issues[0].Evidence, "function=Run;") {
		t.Fatalf("behavioral collaborator versus mapper: %v", issues)
	}
}

func TestTunnelSRPGuardOwnership(t *testing.T) {
	for _, tc := range []struct {
		name, condition string
		want            bool
	}{{"availability", "s.service==nil", true}, {"authorization", "role!=1", false}} {
		t.Run(tc.name, func(t *testing.T) {
			fset, files := parseSource(t, `package p
type Service interface{Handle(int)error}
type Facade struct{service Service}
func(s *Facade)Handle(role int)error{if `+tc.condition+`{return nil};return s.service.Handle(role)}`)
			info := typeCheckSource(t, fset, files)
			for _, decl := range files[0].Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv != nil && guardedForwardingMethod(fn, info) != tc.want {
					t.Fatalf("guard ownership: %s", tc.name)
				}
			}
		})
	}
}
func tunnelAllConfig() Config {
	cfg := DefaultConfig()
	cfg.Profile = ProfileAll
	return cfg
}

func TestTunnelDIPForwardingAndPolicy(t *testing.T) {
	packages := tunnelPackages(t, map[string]string{
		"auth/auth.go": `package auth
type Server struct{}
func(*Server)Lookup(id string)string{return id}
`,
		"consumer/consumer.go": `package consumer
import "example.com/calibration/auth"
type BearerResolver struct{AuthSrv *auth.Server}
func(r *BearerResolver)Resolve(id string)string{if id==""{return "anonymous"};return r.AuthSrv.Lookup(id)}
type SessionResolver struct{AuthSrv *auth.Server}
func(r *SessionResolver)Resolve(id string)string{role:=r.AuthSrv.Lookup(id);if role==""{role="guest"};return role}
type Forward struct{AuthSrv *auth.Server}
func(r *Forward)Resolve(id string)string{return r.AuthSrv.Lookup(id)}
type Many struct{A,B,C,D,E *auth.Server}
func(r *Many)Resolve(id string)string{if id==""{return ""};return r.A.Lookup(id)+r.B.Lookup(id)+r.C.Lookup(id)+r.D.Lookup(id)+r.E.Lookup(id)}
`,
	})
	issues := issuesWithCheck(Run(packages, tunnelAllConfig(), allRulesEnabled()), CheckDIPConcreteDependency)
	for _, owner := range []string{"BearerResolver", "SessionResolver", "Many"} {
		found := false
		for _, issue := range issues {
			found = found || strings.Contains(issue.Evidence, "type="+owner+";")
		}
		if !found {
			t.Errorf("missing %s: %v", owner, issues)
		}
	}
	for _, issue := range issues {
		if strings.Contains(issue.Evidence, "type=Forward;") {
			t.Errorf("forwarding adapter flagged: %v", issue)
		}
	}
}

func TestTunnelTypedDiscriminator(t *testing.T) {
	packages := tunnelPackages(t, map[string]string{
		"domain/format.go": `package domain
type Format string
const(JSON Format="json";Raw Format="raw";Curl Format="curl")
func(f Format)Valid()bool{switch f{case JSON,Raw,Curl:return true;default:return false}}
func(f Format)ContentType()string{switch f{case JSON:return "application/json";case Raw,Curl:return "text/plain";default:return "unknown"}}
func(f Format)Extension()string{switch f{case JSON:return ".json";case Raw:return ".http";case Curl:return ".sh";default:return ""}}
type Config interface{sealed()}
type A struct{};func(A)sealed(){}
func Decode(f Format)(Config,error){switch f{case JSON,Raw,Curl:return A{},nil;default:return nil,nil}}
`,
		"render/render.go": `package render
import("strings";"example.com/calibration/domain")
func Parse(raw string)(domain.Format,error){switch strings.ToLower(raw){case string(domain.JSON):return domain.JSON,nil;case string(domain.Raw):return domain.Raw,nil;case string(domain.Curl):return domain.Curl,nil;default:return "",nil}}
func Build(f domain.Format)string{switch f{case domain.JSON:return "json";case domain.Raw:return "raw";case domain.Curl:return "curl";default:return ""}}
func BuildMany(f domain.Format)string{if f==domain.JSON{return "array"};if f==domain.Raw{return "records"};return "scripts"}
`,
	})
	issues := issuesWithCheck(Run(packages, tunnelAllConfig(), allRulesEnabled()), CheckOCPDiscriminatorDispatch)
	if len(issues) != 1 {
		t.Fatalf("want one typed enum cluster: %v", issues)
	}
	issue := issues[0]
	if !strings.Contains(issue.Evidence, "enum:example.com/calibration/domain.Format") || len(issue.Related) != 4 {
		t.Fatalf("missing five behavioral sites: %+v", issue)
	}
	for _, location := range issue.Related {
		if strings.Contains(location.Message, "Valid") || strings.Contains(location.Message, "Decode") {
			t.Fatalf("control joined cluster: %v", location)
		}
	}
}

func TestTunnelSRPTransportWorkflow(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"nested", "a:=domain.New();h.deps.Store.Create(a);h.deps.Store.Save(a)", 1},
		{"decode-command-encode", "h.deps.Command.Handle()", 0},
		{"single-write", "a:=domain.New();h.deps.Store.Save(a)", 0},
		{"read-only", "h.deps.Store.Get()", 0},
		{"helper", "h.create()", 1},
		{"zero-arg-transition", "a:=h.deps.Store.Get();a.Publish();h.deps.Store.Create(a);h.deps.Store.Save(a)", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packages := tunnelPackages(t, map[string]string{
				"domain/domain.go": `package domain
type Aggregate struct{}
func New()*Aggregate{return &Aggregate{}}
func(*Aggregate)Publish(){}
`,
				"http/handler.go": `package http
import("net/http";"example.com/calibration/domain")
type Store interface{Create(*domain.Aggregate);Save(*domain.Aggregate);Get()*domain.Aggregate}
type Command interface{Handle()}
type Deps struct{Store Store;Command Command}
type Handler struct{deps Deps}
func(h *Handler)ServeHTTP(w http.ResponseWriter,r *http.Request){` + tc.body + `}
func(h *Handler)create(){a:=domain.New();h.deps.Store.Create(a);h.deps.Store.Save(a)}
`,
			})
			issues := issuesWithCheck(Run(packages, tunnelAllConfig(), allRulesEnabled()), CheckSRPTransportWorkflow)
			if len(issues) != tc.want {
				t.Fatalf("got %v want %d", issues, tc.want)
			}
		})
	}
}

func TestTunnelFactoryProductEvidence(t *testing.T) {
	pkgs := tunnelPackages(t, map[string]string{"factory/factory.go": `package factory
 type Product interface{Value()int}
 type A struct{};func(A)Value()int{return 1}
 type B struct{};func(B)Value()int{return 2}
 func NewSame(k int)(Product,error){switch k{case 1:return A{},nil;case 2:return A{},nil;case 3:return A{},nil;case 4:return A{},nil;case 5:return A{},nil};return nil,nil}
 func NewMany(k int)Product{switch k{case 1:return A{};case 2:return B{};case 3:return A{};case 4:return B{};case 5:return A{}};return nil}
 func BuildSame(k int)Product{constructors:=map[int]func()Product{1:func()Product{return A{}},2:func()Product{return A{}},3:func()Product{return A{}},4:func()Product{return A{}},5:func()Product{return A{}}};return constructors[k]()}
 func BuildErrors(k int)(Product,error){constructors:=map[int]func()error{1:func()error{return nil},2:func()error{return nil},3:func()error{return nil},4:func()error{return nil},5:func()error{return nil}};return A{},constructors[k]()}
 func BuildMany(k int)Product{constructors:=map[int]func()Product{1:func()Product{return A{}},2:func()Product{return B{}},3:func()Product{return A{}},4:func()Product{return B{}},5:func()Product{return A{}}};return constructors[k]()}
 `})
	issues := issuesWithCheck(Run(pkgs, tunnelAllConfig(), allRulesEnabled()), CheckOCPClosedFactory)
	if len(issues) != 2 {
		t.Fatalf("product evidence: %v", issues)
	}
	for _, issue := range issues {
		if !strings.Contains(issue.Evidence, "function=NewMany;") && !strings.Contains(issue.Evidence, "function=BuildMany;") {
			t.Fatalf("single/error product flagged: %v", issue)
		}
	}
}
