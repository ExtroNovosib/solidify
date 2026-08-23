package analyzer

import (
	"go/types"
	"strings"
)

const (
	errorsPackagePath = "errors"
	fmtPackagePath    = "fmt"
	ioPackagePath     = "io"
	eofIdentifier     = "EOF"
	errorfFuncName    = "Errorf"
	joinFuncName      = "Join"
	localStructKind   = "struct"
)

func isConfigDataBagType(typeName string) bool {
	return strings.HasSuffix(typeName, "Config")
}

func isSamePackageLocalStruct(kind map[string]string, dep string) bool {
	return kind != nil && kind[dep] == localStructKind
}

func isWellKnownWideInterface(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	switch obj.Pkg().Path() + "." + obj.Name() {
	case "context.Context", "net/http.ResponseWriter":
		return true
	default:
		return false
	}
}
