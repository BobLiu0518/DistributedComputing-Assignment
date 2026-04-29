package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	wd, _ := os.Getwd()
	protoDir := filepath.Join(wd, "..", "..", "..", "proto")
	outDir := filepath.Join(wd, "..", "..", "pb")

	os.MkdirAll(filepath.Join(outDir, "example"), 0755)

	var protoFiles []string
	filepath.Walk(protoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".proto") {
			abs, _ := filepath.Abs(path)
			protoFiles = append(protoFiles, abs)
		}
		return nil
	})

	absProtoDir, _ := filepath.Abs(protoDir)
	absOutDir, _ := filepath.Abs(outDir)

	args := []string{
		"--proto_path=" + absProtoDir,
		"--go_out=" + absOutDir,
		"--go_opt=module=rpc-server/pb",
	}
	args = append(args, protoFiles...)

	cmd := exec.Command("protoc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
