// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa

import (
	"context"
	_ "embed"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

var (
	//go:embed templates/crd-autoscaling.k8s.io_verticalpodautoscalers.yaml
	verticalPodAutoscalerCRD string
	//go:embed templates/crd-autoscaling.k8s.io_verticalpodautoscalercheckpoints.yaml
	verticalPodAutoscalerCheckpointCRD string

	crdResources  map[string]string
	resourceNames []string
)

func init() {
	crdResources = map[string]string{
		"crd-verticalpodautoscalers.yaml":           verticalPodAutoscalerCRD,
		"crd-verticalpodautoscalercheckpoints.yaml": verticalPodAutoscalerCheckpointCRD,
	}
	for _, resource := range crdResources {
		name, err := kubernetesutils.GetObjectNameFromManifest(resource)
		utilruntime.Must(err)
		resourceNames = append(resourceNames, name)
	}
}

type vpaCRD struct {
	client   client.Client
	applier  kubernetes.Applier
	registry *managedresources.Registry
}

// NewCRD can be used to deploy the CRD definitions for the Kubernetes Vertical Pod Autoscaler.
func NewCRD(client client.Client, applier kubernetes.Applier, registry *managedresources.Registry) component.DeployWaiter {
	return &vpaCRD{
		client:   client,
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

// Wait signals whether a CRD is ready or needs more time to be deployed.
func (v *vpaCRD) Wait(ctx context.Context) error {
	return kubernetesutils.WaitUntilCRDManifestsReady(ctx, v.client, resourceNames)
}

// WaitCleanup for destruction to finish and component to be fully removed. crdDeployer does not need to wait for cleanup.
func (v *vpaCRD) WaitCleanup(_ context.Context) error {
	return nil
}
