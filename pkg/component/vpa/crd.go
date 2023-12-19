// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package vpa

import (
	"context"
	_ "embed"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

var (
	//go:embed templates/crd-autoscaling.k8s.io_verticalpodautoscalers.yaml
	verticalPodAutoscalerCRD string
	//go:embed templates/crd-autoscaling.k8s.io_verticalpodautoscalercheckpoints.yaml
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
