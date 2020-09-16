// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package actuator

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/cloudinit"
	commonosgenerator "github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CloudConfigFromOperatingSystemConfig generates a CloudConfig from an OperatingSystemConfig
// using a Generator
func CloudConfigFromOperatingSystemConfig(ctx context.Context, c client.Client, config *extensionsv1alpha1.OperatingSystemConfig, generator commonosgenerator.Generator) ([]byte, *string, error) {
	files := make([]*commonosgenerator.File, 0, len(config.Spec.Files))
	for _, file := range config.Spec.Files {
		data, err := DataForFileContent(ctx, c, config.Namespace, &file.Content)
		if err != nil {
			return nil, nil, err
		}

		files = append(files, &commonosgenerator.File{Path: file.Path, Content: data, Permissions: file.Permissions})
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

	return generator.Generate(&commonosgenerator.OperatingSystemConfig{
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
		return cloudinit.Decode(inline.Encoding, []byte(inline.Data))
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, kutil.Key(namespace, content.SecretRef.Name), secret); err != nil {
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
