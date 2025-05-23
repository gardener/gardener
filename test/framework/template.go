// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/Masterminds/sprig/v3"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// RenderAndDeployTemplate renders a template from the resource template directory and deploys it to the cluster
func (f *CommonFramework) RenderAndDeployTemplate(ctx context.Context, k8sClient kubernetes.Interface, templateName string, values any) error {
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
