// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logcheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/exp/utf8string"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Analyzer is the logcheck analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "logcheck",
	Doc:      "check structured logging calls to logr.Logger instances",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	// find the logger type, so that we can later on check, if a given receiver is actually a logr.Logger instance
	loggerType, err := findLogrLoggerType(pass)
	if err != nil {
		return nil, err
	}
	if loggerType == nil {
		// github.com/go-logr/logr not imported, there are no logs to check
		return nil, nil
	}

	// get a pre-filled *inspector.Inspector, that we requested in Analyzer.Requires
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// filter AST nodes for call expressions (we are only interested in function calls to logr.Logger instances)
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	insp.Preorder(nodeFilter, func(node ast.Node) {
		callExpr := node.(*ast.CallExpr)
		checkCallExpr(pass, callExpr, loggerType)
	})

	return nil, nil
}

func findLogrLoggerType(pass *analysis.Pass) (*types.Named, error) {
	// collect all dependencies of the inspected package by traversing the package import tree
	// then find the logr package
	var logrPkg *types.Package
	for _, pkg := range typeutil.Dependencies(pass.Pkg) {
		if pkg.Path() == "github.com/go-logr/logr" {
			logrPkg = pkg
			break
		}
	}
	if logrPkg == nil {
		// github.com/go-logr/logr not imported
		return nil, nil
	}

	// find Logger type
	obj, ok := logrPkg.Scope().Lookup("Logger").(*types.TypeName)
	if !ok || obj == nil {
		return nil, fmt.Errorf("couldn't find logr.Logger type")
	}
	loggerType, ok := obj.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("expected *types.Named, got %T", obj.Type())
	}
	return loggerType, nil
}

// checkCallExpr checks the given CallExpr for structured logging calls
func checkCallExpr(pass *analysis.Pass, callExpr *ast.CallExpr, loggerType *types.Named) {
	// we are looking for method calls on logr.Logger instances, e.g. log.Info()
	// hence, the function of callExpr must by a SelectorExpr (x.Sel)
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// extracting function Name like Info and eliminate calls to irrelevant functions
	funcName := selExpr.Sel.Name
	switch funcName {
	case "WithValues", "Info", "Error":
	default:
		return
	}

	// now check, if the receiver of our call is actually a logr.Logger instance
	// if not, we don't have to check it in detail
	if sel, ok := pass.TypesInfo.Selections[selExpr]; !ok || !types.AssignableTo(sel.Recv(), loggerType) {
		return
	}

	// we found a relevant logging call, inspect its arguments
	var (
		message       ast.Expr   // nil for WithValues
		keysAndValues []ast.Expr // could be empty
	)

	switch funcName {
	case "WithValues":
		if len(callExpr.Args) == 0 {
			// empty WithValues() call doesn't make any sense
			pass.ReportRangef(selExpr.Sel, "call to %q without arguments", funcName)
			return
		}
		keysAndValues = callExpr.Args
	case "Info":
		if len(callExpr.Args) < 1 {
			// typecheck will complain as well, we don't need to provide helpful advice here, but we should fail nevertheless
			pass.ReportRangef(selExpr.Sel, "call to %q is missing arguments", funcName)
			return
		}
		message = callExpr.Args[0]
		keysAndValues = callExpr.Args[1:]
	case "Error":
		if len(callExpr.Args) < 2 {
			// typecheck will complain as well, we don't need to provide helpful advice here, but we should fail nevertheless
			pass.ReportRangef(selExpr.Sel, "call to %q is missing arguments", funcName)
			return
		}
		message = callExpr.Args[1]
		keysAndValues = callExpr.Args[2:]
	}

	// first check message (if given)
	if message != nil {
		value, ok := isConstantStringExpr(pass, message)
		if !ok {
			pass.ReportRangef(message, "structured logging message should be a constant string expression")
			return
		}

		if value == `""` { // string literal or constant values contain quotes ""
			pass.ReportRangef(message, "structured logging message should not be empty: %s", value)
			return
		}

		// find a common pattern of mistakes: if format specifier is used in message, someone probably forgot that this is
		// structured logging, so check for format specifier first and skip if found
		if specifier, has := hasFormatSpecifier(value); has {
			pass.ReportRangef(message, "structured logging message should not use format specifier %q", specifier)
			return
		}

		if !unicode.IsUpper([]rune(value)[1]) {
			pass.ReportRangef(message, "structured logging message should be capitalized: %s", value)
			return
		}

		if strings.HasSuffix(value, ".\"") {
			pass.ReportRangef(message, "structured logging message should not end with punctuation mark: %s", value)
			return
		}
	}

	// now check the given key-value pairs
	checkKeysAndValues(pass, selExpr.Sel, funcName, keysAndValues)
}

func checkKeysAndValues(pass *analysis.Pass, rng analysis.Range, funcName string, keysAndValues []ast.Expr) {
	// check even number of args
	if len(keysAndValues)%2 != 0 {
		pass.ReportRangef(rng, "structured logging arguments to %s must be key-value pairs, got odd number of arguments: %d", funcName, len(keysAndValues))
		return
	}

	// check keys
	for i, key := range keysAndValues {
		if i%2 != 0 {
			continue
		}

		value, ok := isConstantStringExpr(pass, key)
		if !ok {
			pass.ReportRangef(key, "structured logging key should be a constant string expression")
			continue
		}

		if value == `""` { // string literal or constant values contain quotes ""
			if ident, ok := key.(*ast.Ident); ok {
				value = ident.String()
			}
			pass.ReportRangef(key, "structured logging key should not be empty: %s", value)
			continue
		}

		// find a common pattern of mistakes: if format specifier is used in a key, someone probably forgot that this is
		// structured logging, so check for format specifier first and skip if found
		if specifier, has := hasFormatSpecifier(value); has {
			pass.ReportRangef(key, "structured logging key should not use format specifier %q", specifier)
			return
		}

		if unicode.IsUpper([]rune(value)[1]) {
			pass.ReportRangef(key, "structured logging key should be lowerCamelCase: %s", value)
			continue
		}

		if !utf8string.NewString(value).IsASCII() {
			pass.ReportRangef(key, "structured logging key should be an ASCII string: %s", value)
			continue
		}

		if strings.Contains(value, " ") {
			pass.ReportRangef(key, "structured logging key should not contain spaces: %s", value)
			continue
		}
	}
}

func isConstantStringExpr(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	switch obj := expr.(type) {
	case *ast.BasicLit:
		if obj.Kind == token.STRING {
			return obj.Value, true
		}
	default:
		if typeAndValue, ok := pass.TypesInfo.Types[obj]; ok {
			if basicType, ok := typeAndValue.Type.(*types.Basic); typeAndValue.Value != nil && ok {
				// denotes a string constant or concatenation of string constants, also acceptable
				return typeAndValue.Value.String(), basicType.Kind() == types.String
			}
		}
	}
	return "", false
}

func hasFormatSpecifier(value string) (string, bool) {
	// shortcut for empty string
	if value == "" {
		return "", false
	}

	formatSpecifiers := []string{
		"%v", "%+v", "%#v", "%T",
		"%t", "%b", "%c", "%d", "%o", "%O", "%q", "%x", "%X", "%U",
		"%e", "%E", "%f", "%F", "%g", "%G", "%s", "%q", "%p",
	}
	for _, specifier := range formatSpecifiers {
		if strings.Contains(value, specifier) {
			return specifier, true
		}
	}
	return "", false
}
