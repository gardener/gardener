// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa

import (
	"context"
	_ "embed"
	"maps"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
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
	component.DeployWaiter
	applier  kubernetes.Applier
	registry *managedresources.Registry
}

// NewCRD can be used to deploy the CRD definitions for the Kubernetes Vertical Pod Autoscaler.
func NewCRD(client client.Client, applier kubernetes.Applier, registry *managedresources.Registry) (component.DeployWaiter, error) {
	crdDeployer, err := crddeployer.New(client, applier, slices.Sorted(maps.Values(crdResources)), false)
	if err != nil {
		return nil, err
	}

	return &vpaCRD{
		DeployWaiter: crdDeployer,
		applier:      applier,
		registry:     registry,
	}, nil
}

// Deploy creates and updates the CRD definitions for the Kubernetes Vertical Pod Autoscaler.
func (v *vpaCRD) Deploy(ctx context.Context) error {
	if v.registry != nil {
		for filename, resource := range crdResources {
			v.registry.AddSerialized(filename, []byte(resource))
		}
		return nil
	} else {
		return v.DeployWaiter.Deploy(ctx)
	}
}

func (v *vpaCRD) Destroy(ctx context.Context) error {
	if v.registry != nil {
		// In case of being deployed through a `ManagedResource`, we cannot destroy them here,
		// as the actual deployment happens in another component.
		return nil
	} else {
		return v.DeployWaiter.Destroy(ctx)
	}
}
