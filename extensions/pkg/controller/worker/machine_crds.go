// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"bytes"
	"context"
	"embed"
	"io/fs"
	"path/filepath"
	"text/template"

	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/helm/pkg/engine"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var (
	//go:embed templates/*
	templateContent embed.FS
	machineCRDTpls  []*template.Template

	apiextensionsScheme      = runtime.NewScheme()
	deletionProtectionLabels = map[string]string{
		gardenerutils.DeletionProtected: "true",
	}
)

func init() {
	var err error

	templates, err := fs.Sub(templateContent, "templates")
	utilruntime.Must(err)

	filenames, err := fs.Glob(templates, "*.tpl.yaml")
	utilruntime.Must(err)

	for _, filename := range filenames {
		file, err := templateContent.ReadFile(filepath.Join("templates", filename))
		utilruntime.Must(err)

		tpl := template.Must(
			template.
				New(filename).
				Funcs(engine.FuncMap()).
				Parse(string(file)),
		)

		machineCRDTpls = append(machineCRDTpls, tpl)
	}

	utilruntime.Must(apiextensionsscheme.AddToScheme(apiextensionsScheme))
}

// ApplyMachineResourcesForConfig ensures that all well-known machine CRDs are created or updated.
// Deprecated: This function is deprecated and will be dropped after v1.76 was released. Starting from
// gardener/gardener@v1.73, gardenlet is managing the CRDs for the machine-controller-manager. Hence, extensions do not
// need to take care about it anymore.
// TODO(rfranzke): Remove this function after v1.76 was released.
func ApplyMachineResourcesForConfig(ctx context.Context, config *rest.Config) error {
	c, err := client.New(config, client.Options{Scheme: apiextensionsScheme})
	if err != nil {
		return err
	}

	return ApplyMachineResources(ctx, c)
}

// ApplyMachineResources ensures that all well-known machine CRDs are created or updated.
// Deprecated: This function is deprecated and will be dropped after v1.76 was released. Starting from
// gardener/gardener@v1.73, gardenlet is managing the CRDs for the machine-controller-manager. Hence, extensions do not
// need to take care about it anymore.
// TODO(rfranzke): Remove this function after v1.76 was released.
func ApplyMachineResources(ctx context.Context, c client.Client) error {
	var content bytes.Buffer
	for _, crdTpl := range machineCRDTpls {
		if err := crdTpl.Execute(&content, map[string]interface{}{
			"labels": deletionProtectionLabels,
		}); err != nil {
			return err
		}
		content.Write([]byte("\n---\n"))
	}

	manifestReader := kubernetes.NewManifestReader(content.Bytes())

	applier := kubernetes.NewApplier(c, c.RESTMapper())

	return applier.ApplyManifest(ctx, manifestReader, kubernetes.DefaultMergeFuncs)
}
