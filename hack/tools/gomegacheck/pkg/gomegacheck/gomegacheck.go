// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gomegacheck

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Analyzer is the gomegacheck analyzer.
var Analyzer = &analysis.Analyzer{
	Name:     "gomegacheck",
	Doc:      "check test assertions using gomega",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

var (
	assertionCalls = []string{
		"Should",
		"ShouldNot",
		"To",
		"ToNot",
		"NotTo",
	}
	asyncAssertionCalls = []string{
		"Should",
		"ShouldNot",
	}
	expectedAssertionCalls map[string]struct{}
)

func init() {
	expectedAssertionCalls = make(map[string]struct{}, len(assertionCalls))
	for _, a := range assertionCalls {
		expectedAssertionCalls[a] = struct{}{}
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	// collect all dependencies of the inspected package by traversing the package import tree
	// then find the gomega package
	gomegaPkg := findPackage(pass, "github.com/onsi/gomega")
	if gomegaPkg == nil {
		// github.com/onsi/gomega not imported, there are no assertions to check
		return nil, nil
	}
	typesPkg := findPackage(pass, "github.com/onsi/gomega/types")
	if typesPkg == nil {
		return nil, fmt.Errorf("gomega imported but not gomega/types, this should not happen")
	}

	// find the gomega assertion types in order to identify assertions
	assertionTypes, err := findGomegaAssertionTypes(gomegaPkg)
	if err != nil {
		return nil, err
	}

	// find the gomega matcher type in order to identify matchers
	matcherType, err := findType(typesPkg, "GomegaMatcher")
	if err != nil {
		return nil, err
	}

	// get a pre-filled *inspector.Inspector, that we requested in Analyzer.Requires
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// filter AST nodes for call expressions (we are only interested in function calls to logr.Logger instances)
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
		(*ast.ReturnStmt)(nil),
	}

	for _, assertionType := range assertionTypes {
		insp.Nodes(nodeFilter, func(node ast.Node, push bool) bool {
			if !push {
				return false
			}

			switch e := node.(type) {
			case *ast.CallExpr:
				return checkCallExpr(pass, e, assertionType, matcherType)
			case *ast.ReturnStmt:
				// returning an assertion should be fine generally (e.g. helper funcs)
				// don't traverse further down
				return false
			}
			return true
		})
	}

	return nil, nil
}

func findPackage(pass *analysis.Pass, name string) *types.Package {
	for _, pkg := range typeutil.Dependencies(pass.Pkg) {
		if pkg.Path() == name {
			return pkg
		}
	}
	return nil
}

func findGomegaAssertionTypes(pkg *types.Package) ([]*types.Named, error) {
	assertionType, err := findType(pkg, "Assertion")
	if err != nil {
		return nil, err
	}
	asyncAssertionType, err := findType(pkg, "AsyncAssertion")
	if err != nil {
		return nil, err
	}
	return []*types.Named{assertionType, asyncAssertionType}, nil
}

func findType(pkg *types.Package, name string) (*types.Named, error) {
	obj := pkg.Scope().Lookup(name)
	if obj == nil {
		return nil, fmt.Errorf("couldn't find %s.%s type", pkg.Name(), name)
	}
	typ, ok := obj.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("expected *types.Named, got %T", obj.Type())
	}
	return typ, nil
}

func checkCallExpr(pass *analysis.Pass, callExpr *ast.CallExpr, assertionType *types.Named, matcherType types.Type) bool {
	var assertionExpr ast.Expr

	if isAssertion(pass, callExpr, assertionType) {
		assertionExpr = callExpr
	} else if recv, sel, ok := isCallToAssertion(pass, callExpr, assertionType); ok {
		if _, found := expectedAssertionCalls[sel]; found {
			checkCallToAssertion(pass, callExpr, matcherType)

			// this assertion is correctly handled, don't traverse further down
			return false
		}

		// this is a call to an assertion but not to one of the expected methods
		assertionExpr = recv
	} else {
		// this call does not return an assertion and is not a call to an assertion itself,
		// traverse further down
		return true
	}

	// if we reach here, there was no call to Should and friends up the AST
	missingCalls := assertionCalls
	typ := assertionType.Obj().Name()
	if typ == "AsyncAssertion" {
		missingCalls = asyncAssertionCalls
	}

	pass.Reportf(assertionExpr.End(), "gomega.%s is missing a call to one of %s", typ, strings.Join(missingCalls, ", "))
	return false
}

func checkCallToAssertion(pass *analysis.Pass, callExpr *ast.CallExpr, matcherType types.Type) {
	for i, arg := range callExpr.Args {
		if i == 0 {
			continue
		}

		typesAndInfo, ok := pass.TypesInfo.Types[arg]
		if !ok {
			continue
		}

		if types.AssignableTo(typesAndInfo.Type, matcherType) {
			pass.ReportRangef(arg, "GomegaMatcher passed to optionalDescription param, you can only pass one matcher to each assertion")
		}
	}
}

func isCallToAssertion(pass *analysis.Pass, callExpr *ast.CallExpr, assertionType types.Type) (ast.Expr, string, bool) {
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}

	return selExpr.X, selExpr.Sel.Name, isAssertion(pass, selExpr.X, assertionType)
}

func isAssertion(pass *analysis.Pass, expr ast.Expr, assertionType types.Type) bool {
	typeAndInfo, ok := pass.TypesInfo.Types[expr]
	if !ok {
		return false
	}

	return types.AssignableTo(typeAndInfo.Type, assertionType)
}
