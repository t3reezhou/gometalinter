// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/importer"
	"go/types"
	"io"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/mvdan/interfacer"
	"github.com/mvdan/interfacer/internal/util"
)

var tmpl = template.Must(template.New("std").Parse(`// Generated by generate/std

package interfacer

var stdPkgs = map[string]struct{}{
{{range $_, $pkg := .Pkgs -}}
	"{{$pkg}}": struct{}{},
{{end}}}

var stdIfaces = map[string]string{
{{range $_, $pt := .Ifaces -}}
	"{{$pt.Type}}": "{{$pt.Path}}",
{{end}}}

var stdFuncs = map[string]string{
{{range $_, $pt := .Funcs -}}
	"{{$pt.Type}}": "{{$pt.Path}}",
{{end}}}
`))

var out = flag.String("o", "", "output file")

type byLength []string

func (l byLength) Len() int {
	return len(l)
}

func (l byLength) Less(i, j int) bool {
	if len(l[i]) == len(l[j]) {
		return l[i] < l[j]
	}
	return len(l[i]) < len(l[j])
}

func (l byLength) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

type pkgType struct {
	Type string
	Path string
}

func pkgName(fullname string) string {
	sp := strings.Split(fullname, ".")
	if len(sp) == 1 {
		return ""
	}
	return sp[0]
}

func fullName(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func prepare(in map[string]string, pkgs []string) []pkgType {
	pkgNames := make(map[string][]string)
	nameTypes := make(map[string]string)
	for typestr, fullname := range in {
		path := pkgName(fullname)
		pkgNames[path] = append(pkgNames[path], fullname)
		nameTypes[fullname] = typestr
	}
	var result []pkgType
	addNames := func(path string) {
		names := pkgNames[path]
		sort.Sort(util.ByAlph(names))
		for _, fullname := range names {
			result = append(result, pkgType{
				Type: nameTypes[fullname],
				Path: fullname,
			})
		}
	}
	addNames("")
	for _, path := range pkgs {
		addNames(path)
	}
	return result
}

func generate(w io.Writer, pkgs []string) error {
	ifaces := make(map[string]string)
	funcs := make(map[string]string)
	imp := importer.Default()
	grabTypes := func(path string, scope *types.Scope, all bool) {
		ifs, funs := interfacer.FromScope(scope)
		for iface, name := range ifs {
			if !all && !util.Exported(name) {
				continue
			}
			if _, e := ifaces[iface]; e {
				continue
			}
			ifaces[iface] = fullName(path, name)
		}
		for fun, name := range funs {
			if !all && !util.Exported(name) {
				continue
			}
			if _, e := funcs[fun]; e {
				continue
			}
			funcs[fun] = fullName(path, name)
		}
	}
	grabTypes("", types.Universe, true)
	for _, path := range pkgs {
		if path == "unsafe" {
			continue
		}
		if strings.Contains(path, "internal") {
			continue
		}
		pkg, err := imp.Import(path)
		if err != nil {
			return err
		}
		grabTypes(path, pkg.Scope(), false)
	}
	return tmpl.Execute(w, struct {
		Pkgs          []string
		Ifaces, Funcs []pkgType
	}{
		Pkgs:   pkgs,
		Ifaces: prepare(ifaces, pkgs),
		Funcs:  prepare(funcs, pkgs),
	})
}

func main() {
	flag.Parse()
	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			errExit(err)
		}
		defer f.Close()
		w = f
	}
	var pkgs []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		pkgs = append(pkgs, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		errExit(err)
	}
	sort.Sort(byLength(pkgs))
	if err := generate(w, pkgs); err != nil {
		errExit(err)
	}
}

func errExit(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}