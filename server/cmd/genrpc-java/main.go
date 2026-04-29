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
	path     string
	pkg      string
	javaPkg  string
	services []serviceDef
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

type javaFileData struct {
	Package  string
	Service  string
	Methods  []javaMethod
	ProtoPkg string
}

type javaMethod struct {
	RpcName    string
	JavaName   string
	RequestGo  string
	ResponseGo string
}

var (
	rePackage = regexp.MustCompile(`(?m)^\s*package\s+(\w+)\s*;`)
	reJavaPkg = regexp.MustCompile(`option\s+java_package\s*=\s*"([^"]+)"\s*;`)
	reService = regexp.MustCompile(`(?s)service\s+(\w+)\s*\{([^}]+)\}`)
	reRpc     = regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*([^)\s]+)\s*\)\s*returns\s*\(\s*([^)\s]+)\s*\)\s*;`)
	reComment = regexp.MustCompile(`//[^\n]*|/\*[\s\S]*?\*/`)
)

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	selfDir := filepath.Dir(thisFile)
	protoRoot := filepath.Join(selfDir, "..", "..", "..", "proto", "app")
	outDir := filepath.Join(selfDir, "..", "..", "..", "client", "src", "main", "java", "tech", "bobliu", "rpc", "app")
	os.MkdirAll(outDir, 0755)

	count := 0
	filepath.Walk(protoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".proto") {
			return err
		}
		pf := parse(path)
		for _, svc := range pf.services {
			data := javaFileData{
				Package:  "tech.bobliu.rpc.app",
				Service:  svc.name,
				ProtoPkg: pf.javaPkg,
			}
			for _, m := range svc.methods {
				reqType := typeName(m.requestType)
				respType := typeName(m.responseType)
				data.Methods = append(data.Methods, javaMethod{
					RpcName:    m.name,
					JavaName:   lowerFirst(m.name),
					RequestGo:  reqType,
					ResponseGo: respType,
				})
			}
			outPath := filepath.Join(outDir, svc.name+"Rpc.java")
			f, err := os.Create(outPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "genrpc-java: %v\n", err)
				continue
			}
			javaTmpl.Execute(f, data)
			f.Close()
			count++
			fmt.Printf("genrpc-java: wrote %s\n", outPath)
		}
		return nil
	})

	if count == 0 {
		fmt.Println("genrpc-java: no service definitions found")
	}
}

func parse(path string) protoFile {
	data, _ := os.ReadFile(path)
	content := reComment.ReplaceAllString(string(data), "")
	pf := protoFile{path: path}
	if m := rePackage.FindStringSubmatch(content); m != nil {
		pf.pkg = m[1]
	}
	if m := reJavaPkg.FindStringSubmatch(content); m != nil {
		pf.javaPkg = m[1]
	}
	for _, sm := range reService.FindAllStringSubmatch(content, -1) {
		svc := serviceDef{name: sm[1]}
		for _, rm := range reRpc.FindAllStringSubmatch(sm[2], -1) {
			svc.methods = append(svc.methods, methodDef{
				name:         rm[1],
				requestType:  strings.TrimPrefix(rm[2], "."),
				responseType: strings.TrimPrefix(rm[3], "."),
			})
		}
		pf.services = append(pf.services, svc)
	}
	return pf
}

func typeName(s string) string {
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

var javaTmpl = template.Must(template.New("java").Parse(`package {{.Package}};

import {{.ProtoPkg}}.*;
import tech.bobliu.rpc.annotation.RpcMethod;
import tech.bobliu.rpc.annotation.RpcService;

@RpcService("{{.Service}}")
public interface {{.Service}}Rpc {
{{- range .Methods}}
    @RpcMethod(requestType = {{.RequestGo}}.class, responseType = {{.ResponseGo}}.class)
    {{.ResponseGo}} {{.JavaName}}({{.RequestGo}} request);
{{- end}}
}
`))
