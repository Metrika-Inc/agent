package main

import (
	"bufio"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed node.go.template
var nodeTmpl []byte

const (
	nodeTmplFile = "node.go.template"
)

var (
	blockchain  string
	defaultPath string
	outPath     string
	outDir      string
	srcPath     string
)

func init() {
	flag.StringVar(&blockchain, "blockchain", "", "Blockchain type to render bindings for.")
	flag.Parse()

	srcPath = os.Getenv("MA_SRC_PATH")
	if len(srcPath) == 0 {
		log.Fatalf("MA_SRC_PATH not set (i.e. MA_SRC_PATH=/home/me/src/agent)")
	}

	switch blockchain {
	case "":
		log.Fatalf("-blockchain is required (i.e. flow)")
	case "flow":
	default:
		log.Fatalf("no bindings available for protocol %q", blockchain)
	}

	defaultPath = filepath.Join(srcPath, "protobind", "node.go.template")
	defaultOutDir := filepath.Join(srcPath, "internal", "pkg", "discover")

	outPath = strings.TrimSuffix(nodeTmplFile, ".template")
	outPath = filepath.Join(defaultOutDir, fmt.Sprintf("node_%s.go", blockchain))
}

func main() {
	if _, err := os.Stat(defaultPath); err != nil {
		log.Fatal(err)
	}

	funcMap := template.FuncMap{
		"ToUpper": strings.ToUpper,
		"Title":   strings.Title,
	}

	tmpl := template.Must(template.New("protobind").Funcs(funcMap).Parse(string(nodeTmpl)))

	conf := struct {
		Blockchain string
	}{
		Blockchain: blockchain,
	}

	f, err := os.Create(outPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	if err := tmpl.Execute(w, conf); err != nil {
		log.Fatalf("execution failed: %s", err)
	}
	w.Flush()
}
