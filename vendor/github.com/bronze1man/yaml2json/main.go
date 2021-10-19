package main

import (
	"os"

	"github.com/bronze1man/yaml2json/y2jLib"
)

	func main() {
	if len(os.Args)>1 && os.Args[1]=="--help"{
		os.Stdout.WriteString(`Transform yaml string to json string without the type infomation.
Usage:
echo "a: 1" | yaml2json
yaml2json < 1.yml > 2.json
`)
		os.Exit(1)
		return
	}
	err := y2jLib.TranslateStream(os.Stdin, os.Stdout)
	if err == nil{
		os.Exit(0)
	}
	os.Stderr.WriteString(err.Error())
	os.Stderr.WriteString("\n")
	os.Exit(2)
}

