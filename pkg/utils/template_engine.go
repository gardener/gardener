// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"
)

const templateDir = "templates"

var standardFunctions = template.FuncMap{
	"indent": func(spaces int, v string) string {
		pad := strings.Repeat(" ", spaces)
		return pad + strings.Replace(v, "\n", "\n"+pad, -1)
	},
}

// RenderTemplate reads the template file in the <templateDir> directory and renders it. It injects a bunch
// of standard functions which can be used in the template file.
func RenderTemplate(filename string, values interface{}) ([]byte, error) {
	return RenderTemplateWithFuncs(filename, standardFunctions, values)
}

// RenderTemplateWithFuncs reads the template file in the <templateDir> directory and renders it. It allows
// providing a user-defined template.FuncMap <funcs> to the template which will be merged with the standard
// functions and provided to the template file. The user-defined functions always take precedence in the
// merge process.
func RenderTemplateWithFuncs(filename string, funcs template.FuncMap, values interface{}) ([]byte, error) {
	return RenderTemplatesWithFuncs([]string{filename}, funcs, values)
}

// RenderTemplatesWithFuncs does the same as RenderTemplateWithFuncs except that it allows providing multiple
// template files instead of only exactly one.
func RenderTemplatesWithFuncs(filenames []string, funcs template.FuncMap, values interface{}) ([]byte, error) {
	var paths []string
	for _, filename := range filenames {
		paths = append(paths, filepath.Join(templateDir, filename))
	}

	templateObj, err := template.
		New(filenames[0][strings.LastIndex(filenames[0], "/")+1:]).
		Funcs(mergeFunctions(funcs)).
		ParseFiles(paths...)
	if err != nil {
		return nil, err
	}
	return render(templateObj, values)
}

// RenderLocalTemplate uses a template <tpl> given as a string and renders it. Thus, the template does not
// necessarily need to be stored as a file.
func RenderLocalTemplate(tpl string, values interface{}) ([]byte, error) {
	templateObj, err := template.
		New("tpl").
		Parse(tpl)
	if err != nil {
		return nil, err
	}
	return render(templateObj, values)
}

// render takes a text/template.Template object <temp> and an interface of <values> which are used to render the
// template. It returns the rendered result as byte slice, or an error if something went wrong.
func render(tpl *template.Template, values interface{}) ([]byte, error) {
	var result bytes.Buffer
	err := tpl.Execute(&result, values)
	if err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

// mergeFunctions takes a template.FuncMap <funcs> and merges them with the standard functions. If <funcs>
// defines a function with a name already existing in the standard functions map, the standard function will
// be overwritten.
func mergeFunctions(funcs template.FuncMap) template.FuncMap {
	var functions = template.FuncMap{}
	for i, function := range standardFunctions {
		functions[i] = function
	}
	for i, function := range funcs {
		functions[i] = function
	}
	return functions
}
