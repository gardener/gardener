// Copyright 2019 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package framework

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/Masterminds/sprig"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// RenderAndDeployTemplate renders a template from the resource template directory and deploys it to the cluster
func (f *CommonFramework) RenderAndDeployTemplate(ctx context.Context, k8sClient kubernetes.Interface, templateName string, values interface{}) error {
	templateFilepath := filepath.Join(f.TemplatesDir, templateName)
	if _, err := os.Stat(templateFilepath); err != nil {
		return fmt.Errorf("could not find template in %q", templateFilepath)
	}

	tpl, err := template.
		New(templateName).
		Funcs(sprig.HtmlFuncMap()).
		ParseFiles(templateFilepath)
	if err != nil {
		return fmt.Errorf("unable to parse template in %s: %w", templateFilepath, err)
	}

	var writer bytes.Buffer
	err = tpl.Execute(&writer, values)
	if err != nil {
		return err
	}

	manifestReader := kubernetes.NewManifestReader(writer.Bytes())
	return k8sClient.Applier().ApplyManifest(ctx, manifestReader, kubernetes.DefaultMergeFuncs)
}
