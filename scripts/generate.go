package main

import (
	"bytes"
	"log"
	"os"
	"strings"

	"github.com/traefik/yaegi/extract"
)

func main() {
outer:
	for {
		files, err := os.ReadDir(".")
		if err != nil {
			log.Fatalln(err)
		}
		for _, file := range files {
			if file.Name() == "go.mod" {
				break outer
			}
		}
		err = os.Chdir("..")
		if err != nil {
			log.Fatalln(err)
		}
	}

	err := extractKopyashipPkg()
	if err != nil {
		log.Fatalln(err)
	}
	err = extractGitHubPkg("github.com/mitchellh/go-homedir")
	if err != nil {
		log.Fatalln(err)
	}
}

func extractGitHubPkg(gitHubPath string) error {
	var (
		pkgIdent, importPath = gitHubPath, gitHubPath
		ext                  = extract.Extractor{Dest: "symbols"}
		b                    = bytes.Buffer{}
		gitHubSuffix         = gitHubSuffix(importPath)
	)

	_, err := ext.Extract(pkgIdent, importPath, &b)
	if err != nil {
		return nil
	}
	return os.WriteFile("./internal/scripting/symbols/symbols_"+gitHubSuffix+".go", b.Bytes(), 0644)
}

func extractKopyashipPkg() error {
	var (
		ext = extract.Extractor{Dest: "symbols"}
		b   = bytes.Buffer{}
	)

	_, err := ext.Extract(".", "github.com/tomruk/kopyaship", &b)
	if err != nil {
		return nil
	}
	buf := b.Bytes()
	buf = bytes.Replace(buf, []byte("	\".\"\n"), nil, 1)
	return os.WriteFile("./internal/scripting/symbols/symbols_kopyaship.go", buf, 0644)
}

func gitHubSuffix(path string) string {
	path = strings.TrimPrefix(path, "github.com/")
	return strings.ReplaceAll(path, "/", "_")
}
