// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package main

import (
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/gardener/gardener/landscaper/common/generate"
	importsv1alpha1 "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/generate/openapi"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	tplName = "blueprint-controlplane"
	//go:embed templates/blueprint-controlplane.tpl.yaml
	tplContent string
	tpl        *template.Template
)

func init() {
	var err error
	tpl, err = template.
		New(tplName).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContent)
	if err != nil {
		panic(err)
	}
}

func main() {
	scheme := runtime.NewScheme()
	if err := importsv1alpha1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	if err := generate.RenderBlueprint(tpl,
		scheme,
		"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1.Imports",
		openapi.GetOpenAPIDefinitions,
		"landscaper/pkg/controlplane/blueprint",
	); err != nil {
		panic(err)
	}
}
