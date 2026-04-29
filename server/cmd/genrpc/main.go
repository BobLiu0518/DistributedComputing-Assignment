// genrpc reads protobuf service definitions from proto/ and generates
// strongly-typed Go handler interfaces + automatic router registration.
//
// Usage:
//
//	go generate ./...
//
// The generated file is written to internal/rpc/handlers_gen.go.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template"
)

type protoFile struct {
	path      string
	pkg       string
	goPackage string
	services  []serviceDef
}

type serviceDef struct {
	name    string
	methods []methodDef
}

type methodDef struct {
	name         string
	requestType  string
	responseType string
}

type goType struct {
	pkgPath  string
	pkgName  string
	typeName string
}

type templateData struct {
	Imports  []importEntry
	Services []serviceTemplateData
}

type importEntry struct {
	Alias string
	Path  string
}

type serviceTemplateData struct {
	ServiceName string
	HandlerName string
	Methods     []methodTemplateData
}

type methodTemplateData struct {
	RpcName    string
	RouterKey  string
	RequestGo  string
	ResponseGo string
	RequestNew string
}

var (
	rePackage   = regexp.MustCompile(`(?m)^\s*package\s+(\w+)\s*;`)
	reGoPackage = regexp.MustCompile(`option\s+go_package\s*=\s*"([^"]+)"\s*;`)
	reService   = regexp.MustCompile(`(?s)service\s+(\w+)\s*\{([^}]+)\}`)
	reRpc       = regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*([^)\s]+)\s*\)\s*returns\s*\(\s*([^)\s]+)\s*\)\s*;`)
	reComment   = regexp.MustCompile(`//[^\n]*|/\*[\s\S]*?\*/`)
)

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	selfDir := filepath.Dir(thisFile)
	protoRoot := filepath.Join(selfDir, "..", "..", "..", "proto")
	outPath := filepath.Join(selfDir, "..", "..", "internal", "rpc", "handlers_gen.go")

	var protoFiles []protoFile
	filepath.Walk(protoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".proto") {
			pf, err := parseProto(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "genrpc: parse %s: %v\n", path, err)
				return nil
			}
			if len(pf.services) > 0 {
				protoFiles = append(protoFiles, pf)
			}
		}
		return nil
	})

	if len(protoFiles) == 0 {
		os.MkdirAll(filepath.Dir(outPath), 0755)
		os.WriteFile(outPath, []byte(emptyGenFile), 0644)
		return
	}

	pkgMap := map[string]string{}
	for _, pf := range protoFiles {
		if pf.pkg != "" && pf.goPackage != "" {
			pkgMap[pf.pkg] = pf.goPackage
		}
	}

	data := templateData{}
	importSet := map[string]bool{}

	for _, pf := range protoFiles {
		for _, svc := range pf.services {
			svcData := serviceTemplateData{
				ServiceName: svc.name,
				HandlerName: svc.name + "Handler",
			}
			for _, m := range svc.methods {
				reqType := resolveType(m.requestType, pf.pkg, pkgMap)
				respType := resolveType(m.responseType, pf.pkg, pkgMap)

				alias := toAlias(reqType.pkgPath)
				importSet[keyFor(alias, reqType.pkgPath)] = true

				alias2 := toAlias(respType.pkgPath)
				importSet[keyFor(alias2, respType.pkgPath)] = true

				svcData.Methods = append(svcData.Methods, methodTemplateData{
					RpcName:    m.name,
					RouterKey:  lowerFirst(m.name),
					RequestGo:  alias + "." + reqType.typeName,
					ResponseGo: alias2 + "." + respType.typeName,
					RequestNew: "&" + alias + "." + reqType.typeName + "{}",
				})
			}
			data.Services = append(data.Services, svcData)
		}
	}

	for key := range importSet {
		parts := strings.SplitN(key, " ", 2)
		data.Imports = append(data.Imports, importEntry{Alias: parts[0], Path: parts[1]})
	}

	os.MkdirAll(filepath.Dir(outPath), 0755)

	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genrpc: create output: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := genTemplate.Execute(f, data); err != nil {
		fmt.Fprintf(os.Stderr, "genrpc: template: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("genrpc: wrote %s (%d service(s))\n", outPath, len(data.Services))
}

func parseProto(path string) (protoFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return protoFile{}, err
	}
	content := reComment.ReplaceAllString(string(data), "")

	pf := protoFile{path: path}

	if m := rePackage.FindStringSubmatch(content); m != nil {
		pf.pkg = m[1]
	}
	if m := reGoPackage.FindStringSubmatch(content); m != nil {
		pf.goPackage = m[1]
	}

	for _, svcMatch := range reService.FindAllStringSubmatch(content, -1) {
		svc := serviceDef{name: svcMatch[1]}
		for _, rpcMatch := range reRpc.FindAllStringSubmatch(svcMatch[2], -1) {
			svc.methods = append(svc.methods, methodDef{
				name:         rpcMatch[1],
				requestType:  stripDotPrefix(rpcMatch[2]),
				responseType: stripDotPrefix(rpcMatch[3]),
			})
		}
		pf.services = append(pf.services, svc)
	}

	return pf, nil
}

func resolveType(typeName, currentPkg string, pkgMap map[string]string) goType {
	if idx := strings.LastIndex(typeName, "."); idx >= 0 {
		pkg := typeName[:idx]
		name := typeName[idx+1:]
		goPkgPath := pkgMap[pkg]
		if goPkgPath == "" {
			goPkgPath = "rpc-server/pb/" + pkg
		}
		return goType{pkgPath: goPkgPath, pkgName: lastPathSegment(goPkgPath), typeName: name}
	}

	goPkgPath := pkgMap[currentPkg]
	if goPkgPath == "" {
		goPkgPath = "rpc-server/pb/" + currentPkg
	}
	return goType{pkgPath: goPkgPath, pkgName: lastPathSegment(goPkgPath), typeName: typeName}
}

func stripDotPrefix(s string) string { return strings.TrimPrefix(s, ".") }

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func lastPathSegment(path string) string {
	path = strings.TrimRight(path, "/")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func toAlias(path string) string {
	base := lastPathSegment(path)
	if base == "rpc" {
		return "rpcproto"
	}
	return "pb_" + base
}

func keyFor(alias, path string) string { return alias + " " + path }

var genTemplate = template.Must(template.New("gen").Parse(`// Code generated by genrpc; DO NOT EDIT.

package rpc

import (
	"context"

	"google.golang.org/protobuf/proto"

	"rpc-server/internal/router"
{{range .Imports}}	{{.Alias}} "{{.Path}}"
{{end}})
{{range .Services}}
type {{.HandlerName}} interface {
{{- range .Methods}}
	{{.RpcName}}(ctx context.Context, req *{{.RequestGo}}) (*{{.ResponseGo}}, error)
{{- end}}
}

func Register{{.ServiceName}}(r *router.Router, svc {{.HandlerName}}) {
{{- $svc := .}}
{{- range .Methods}}
	r.Register("{{$svc.ServiceName}}", "{{.RouterKey}}", func(ctx context.Context, payload []byte) ([]byte, error) {
		req := {{.RequestNew}}
		if err := proto.Unmarshal(payload, req); err != nil {
			return nil, err
		}
		resp, err := svc.{{.RpcName}}(ctx, req)
		if err != nil {
			return nil, err
		}
		return proto.Marshal(resp)
	})
{{- end}}
}
{{end}}`))

const emptyGenFile = `// Code generated by genrpc; DO NOT EDIT.

package rpc

// No protobuf service definitions found.
`
