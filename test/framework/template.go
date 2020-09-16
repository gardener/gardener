// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/mock/go/context"
	"github.com/pkg/errors"
)

// RenderAndDeployTemplate renders a template from the resource template directory and deploys it to the cluster
func (f *CommonFramework) RenderAndDeployTemplate(ctx context.Context, k8sClient kubernetes.Interface, templateName string, values interface{}) error {
	templateFilepath := filepath.Join(f.TemplatesDir, templateName)
	if _, err := os.Stat(templateFilepath); err != nil {
		return errors.Errorf("could not find template in '%s'", templateFilepath)
	}

	tpl, err := template.ParseFiles(templateFilepath)
	if err != nil {
		return errors.Wrapf(err, "unable to parse template in %s", templateFilepath)
	}

	var writer bytes.Buffer
	err = tpl.Execute(&writer, values)
	if err != nil {
		return err
	}

	manifestReader := kubernetes.NewManifestReader(writer.Bytes())
	return k8sClient.Applier().ApplyManifest(ctx, manifestReader, kubernetes.DefaultMergeFuncs)
}
