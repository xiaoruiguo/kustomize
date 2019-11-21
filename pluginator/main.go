// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

// A code generator. See /plugin/doc.go for an explanation.
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/provenance"
)

//go:generate stringer -type=pluginType
type pluginType int

const (
	unknown pluginType = iota
	Transformer
	Generator
)

const packageForGeneratedCode = "builtins"

func main() {
	root := inputFileRoot()
	file, err := os.Open(root + ".go")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	readToPackageMain(scanner, file.Name())

	w := NewWriter(root)
	defer w.close()

	// This particular phrasing is required.
	w.write(
		fmt.Sprintf(
			"// Code generated by pluginator on %s; DO NOT EDIT.",
			root))
	w.write(
		fmt.Sprintf(
			"// pluginator %s\n", provenance.GetProvenance().Short()))
	w.write("\n")
	w.write("package " + packageForGeneratedCode)

	pType := unknown

	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, "//go:generate") {
			continue
		}
		if strings.HasPrefix(l, "//noinspection") {
			continue
		}
		if l == "var "+konfig.PluginSymbol+" plugin" {
			continue
		}
		if strings.Contains(l, " Transform(") {
			if pType != unknown {
				log.Fatal("unexpected Transform(")
			}
			pType = Transformer
		} else if strings.Contains(l, " Generate(") {
			if pType != unknown {
				log.Fatal("unexpected Generate(")
			}
			pType = Generator
		}
		w.write(l)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	w.write("")
	w.write("func New" + root + "Plugin() resmap." + pType.String() + "Plugin {")
	w.write("  return &" + root + "Plugin{}")
	w.write("}")
}

func inputFileRoot() string {
	n := os.Getenv("GOFILE")
	if !strings.HasSuffix(n, ".go") {
		log.Printf("%+v\n", provenance.GetProvenance())
		log.Fatalf("expecting .go suffix on %s", n)
	}
	return n[:len(n)-len(".go")]
}

func readToPackageMain(s *bufio.Scanner, f string) {
	gotMain := false
	for !gotMain && s.Scan() {
		gotMain = strings.HasPrefix(s.Text(), "package main")
	}
	if !gotMain {
		log.Fatalf("%s missing package main", f)
	}
}

type writer struct {
	root string
	f    *os.File
}

func NewWriter(r string) *writer {
	n := makeOutputFileName(r)
	f, err := os.Create(n)
	if err != nil {
		log.Fatalf("unable to create `%s`; %v", n, err)
	}
	return &writer{root: r, f: f}
}

// Assume that this command is running with a $PWD of
//   $HOME/kustomize/plugin/builtin/secretgenerator
// (for example).  Then we want to write to
//   $HOME/kustomize/api/builtins
func makeOutputFileName(root string) string {
	return filepath.Join(
		"..", "..", "..", "api", packageForGeneratedCode, root+".go")
}

func (w *writer) close() {
	// Do this for debugging.
	// fmt.Println("Generated " + makeOutputFileName(w.root))
	w.f.Close()
}

func (w *writer) write(line string) {
	_, err := w.f.WriteString(w.filter(line) + "\n")
	if err != nil {
		log.Printf("Trouble writing: %s", line)
		log.Fatal(err)
	}
}

func (w *writer) filter(in string) string {
	if ok, newer := w.replace(in, "type plugin struct"); ok {
		return newer
	}
	if ok, newer := w.replace(in, "*plugin)"); ok {
		return newer
	}
	return in
}

// replace 'plugin' with 'FooPlugin' in context
// sensitive manner.
func (w *writer) replace(in, target string) (bool, string) {
	if !strings.Contains(in, target) {
		return false, ""
	}
	newer := strings.Replace(
		target, "plugin", w.root+"Plugin", 1)
	return true, strings.Replace(in, target, newer, 1)
}
