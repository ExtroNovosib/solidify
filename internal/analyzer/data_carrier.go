package analyzer

import (
	"go/ast"
	"strings"
)

func serializedStructFieldCounts(st *ast.StructType) (tagged, total int) {
	if st == nil || st.Fields == nil {
		return 0, 0
	}
	for _, field := range st.Fields.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		total += count
		if hasSerializationTag(field.Tag) {
			tagged += count
		}
	}
	return tagged, total
}

func hasSerializationTag(tag *ast.BasicLit) bool {
	if tag == nil {
		return false
	}
	for _, key := range []string{"json", "yaml", "toml", "xml", "mapstructure"} {
		if strings.Contains(tag.Value, key+":\"") {
			return true
		}
	}
	return false
}

func isSerializedDataCarrier(taggedFields, totalFields int, methods []*ast.FuncDecl) bool {
	if totalFields < 2 || taggedFields*2 < totalFields {
		return false
	}
	if len(methods) == 0 {
		return true
	}
	trivial := 0
	for _, method := range methods {
		if isTrivialAccessorMethod(method) {
			trivial++
		}
	}
	return trivial*3 >= len(methods)*2
}

func serializedDataCarrierStruct(files []*ast.File, typeName string, st *ast.StructType) bool {
	tagged, total := serializedStructFieldCounts(st)
	if total < 2 || tagged*2 < total {
		return false
	}
	var methods []*ast.FuncDecl
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			if receiverTypeName(fn.Recv.List[0].Type) == typeName {
				methods = append(methods, fn)
			}
		}
	}
	return isSerializedDataCarrier(tagged, total, methods)
}
