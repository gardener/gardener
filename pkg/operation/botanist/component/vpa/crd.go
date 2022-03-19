// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpa

import (
	"context"
	_ "embed"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

var (
	//go:embed templates/crd-verticalpodautoscalers.tpl.yaml
	verticalPodAutoscalerCRD string
	//go:embed templates/crd-verticalpodautoscalercheckpoints.tpl.yaml
	verticalPodAutoscalerCheckpointCRD string

	crdResources map[string]string
)

func init() {
	crdResources = map[string]string{
		"crd-verticalpodautoscalers.yaml":           verticalPodAutoscalerCRD,
		"crd-verticalpodautoscalercheckpoints.yaml": verticalPodAutoscalerCheckpointCRD,
	}
}

type vpaCRD struct {
	applier  kubernetes.Applier
	registry *managedresources.Registry
}

// NewCRD can be used to deploy the CRD definitions for the Kubernetes Vertical Pod Autoscaler.
func NewCRD(applier kubernetes.Applier, registry *managedresources.Registry) component.Deployer {
	return &vpaCRD{
		applier:  applier,
		registry: registry,
	}
}

// Deploy creates and updates the CRD definitions for the Kubernetes Vertical Pod Autoscaler.
func (v *vpaCRD) Deploy(ctx context.Context) error {
	for filename, resource := range crdResources {
		if v.registry != nil {
			v.registry.AddSerialized(filename, []byte(resource))
			continue
		}

		if err := v.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(resource)), kubernetes.DefaultMergeFuncs); err != nil {
			return err
		}
	}

	return nil
}

func (v *vpaCRD) Destroy(ctx context.Context) error {
	if v.registry != nil {
		return nil
	}

	for _, crd := range crdResources {
		if err := v.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(crd))); err != nil {
			return err
		}
	}

	return nil
}
