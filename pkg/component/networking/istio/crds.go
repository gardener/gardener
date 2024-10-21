// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"context"
	"embed"
	"path/filepath"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	//go:embed charts/istio/istio-crds
	chartCRDs     embed.FS
	chartPathCRDs = filepath.Join("charts", "istio", "istio-crds")
)

type crds struct {
	kubernetes.ChartApplier
	client   client.Client
	crdNames []string
}

// NewCRD can be used to deploy istio CRDs.
func NewCRD(
	client client.Client,
	applier kubernetes.ChartApplier,
) component.DeployWaiter {
	deployWaiter := &crds{
		ChartApplier: applier,
		client:       client,
	}

	chart, err := deployWaiter.RenderEmbeddedFS(chartCRDs, chartPathCRDs, "", "", nil)
	utilruntime.Must(err)

	for _, manifest := range chart.Manifests {
		obj, err := kubernetes.NewManifestReader([]byte(manifest.Content)).Read()
		utilruntime.Must(err)

		deployWaiter.crdNames = append(deployWaiter.crdNames, obj.GetName())
	}
	return deployWaiter
}

func (c *crds) Deploy(ctx context.Context) error {
	return c.ApplyFromEmbeddedFS(ctx, chartCRDs, chartPathCRDs, "", "istio")
}

func (c *crds) Destroy(ctx context.Context) error {
	return c.DeleteFromEmbeddedFS(ctx, chartCRDs, chartPathCRDs, "", "istio")
}

func (c *crds) Wait(ctx context.Context) error {
	return kubernetesutils.WaitUntilCRDManifestsReady(ctx, c.client, c.crdNames)
}

func (c *crds) WaitCleanup(_ context.Context) error {
	return nil
}
