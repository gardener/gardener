// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package actuator

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonosgenerator "github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// CloudConfigFromOperatingSystemConfig generates a CloudConfig from an OperatingSystemConfig
// using a Generator
func CloudConfigFromOperatingSystemConfig(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	config *extensionsv1alpha1.OperatingSystemConfig,
	generator commonosgenerator.Generator,
) (
	[]byte,
	*string,
	error,
) {
	files := make([]*commonosgenerator.File, 0, len(config.Spec.Files))
	for _, file := range config.Spec.Files {
		data, err := DataForFileContent(ctx, c, config.Namespace, &file.Content)
		if err != nil {
			return nil, nil, err
		}

		files = append(files, &commonosgenerator.File{Path: file.Path, Content: data, Permissions: file.Permissions, TransmitUnencoded: file.Content.TransmitUnencoded})
	}

	units := make([]*commonosgenerator.Unit, 0, len(config.Spec.Units))
	for _, unit := range config.Spec.Units {
		var content []byte
		if unit.Content != nil {
			content = []byte(*unit.Content)
		}

		dropIns := make([]*commonosgenerator.DropIn, 0, len(unit.DropIns))
		for _, dropIn := range unit.DropIns {
			dropIns = append(dropIns, &commonosgenerator.DropIn{Name: dropIn.Name, Content: []byte(dropIn.Content)})
		}
		units = append(units, &commonosgenerator.Unit{Name: unit.Name, Content: content, DropIns: dropIns})
	}

	return generator.Generate(log, &commonosgenerator.OperatingSystemConfig{
		Object:    config,
		Bootstrap: config.Spec.Purpose == extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
		CRI:       config.Spec.CRIConfig,
		Files:     files,
		Units:     units,
		Path:      config.Spec.ReloadConfigFilePath,
	})
}

// DataForFileContent returns the content for a FileContent, retrieving from a Secret if necessary.
func DataForFileContent(ctx context.Context, c client.Client, namespace string, content *extensionsv1alpha1.FileContent) ([]byte, error) {
	if inline := content.Inline; inline != nil {
		if len(inline.Encoding) == 0 {
			return []byte(inline.Data), nil
		}
		return extensionsv1alpha1helper.Decode(inline.Encoding, []byte(inline.Data))
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, kubernetesutils.Key(namespace, content.SecretRef.Name), secret); err != nil {
		return nil, err
	}

	return secret.Data[content.SecretRef.DataKey], nil
}

// OperatingSystemConfigUnitNames returns the names of the units in the OperatingSystemConfig
func OperatingSystemConfigUnitNames(config *extensionsv1alpha1.OperatingSystemConfig) []string {
	unitNames := make([]string, 0, len(config.Spec.Units))
	for _, unit := range config.Spec.Units {
		unitNames = append(unitNames, unit.Name)
	}
	return unitNames
}
