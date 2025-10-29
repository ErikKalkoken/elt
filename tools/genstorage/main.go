// genstorage generates Go code that implements storage methods for Eve objects.
package main

import (
	_ "embed"
	"flag"
	"html/template"
	"log"
	"os"
	"slices"
)

//go:embed target.go.template
var tmpl string

var packageFlag = flag.String("p", "main", "package name")

func main() {
	flag.Parse()
	if len(flag.Args()) == 0 {
		log.Fatal("Need to specify at least one object name")
	}
	objects := flag.Args()
	slices.Sort(objects)
	tmpl, err := template.New("").Parse(tmpl)
	if err != nil {
		log.Fatal(err)
	}
	f, _ := os.Create("storage_gen.go")
	defer f.Close()
	data := map[string]any{
		"Package": *packageFlag,
		"Objects": objects,
	}
	if err := tmpl.Execute(f, data); err != nil {
		log.Fatal(err)
	}
}
