// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheusoperator

import (
	"context"
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	//go:embed templates/crd-monitoring.coreos.com_alertmanagerconfigs.yaml
	crdAlertmanagerConfigs string
	//go:embed templates/crd-monitoring.coreos.com_alertmanagers.yaml
	crdAlertmanagers string
	//go:embed templates/crd-monitoring.coreos.com_podmonitors.yaml
	crdPodMonitors string
	//go:embed templates/crd-monitoring.coreos.com_probes.yaml
	crdProbes string
	//go:embed templates/crd-monitoring.coreos.com_prometheusagents.yaml
	crdPrometheusAgents string
	//go:embed templates/crd-monitoring.coreos.com_prometheuses.yaml
	crdPrometheuses string
	//go:embed templates/crd-monitoring.coreos.com_prometheusrules.yaml
	crdPrometheusRules string
	//go:embed templates/crd-monitoring.coreos.com_scrapeconfigs.yaml
	crdScrapeConfigs string
	//go:embed templates/crd-monitoring.coreos.com_servicemonitors.yaml
	crdServiceMonitors string
	//go:embed templates/crd-monitoring.coreos.com_thanosrulers.yaml
	crdThanosRulers string

	resources []string
)

func init() {
	resources = append(resources,
		crdAlertmanagerConfigs,
		crdAlertmanagers,
		crdPodMonitors,
		crdProbes,
		crdPrometheusAgents,
		crdPrometheuses,
		crdPrometheusRules,
		crdScrapeConfigs,
		crdServiceMonitors,
		crdThanosRulers,
	)
}

type crdDeployer struct {
	client  client.Client
	applier kubernetes.Applier
}

// NewCRDs can be used to deploy the CRD definitions for the Prometheus Operator.
func NewCRDs(client client.Client, applier kubernetes.Applier) component.DeployWaiter {
	return &crdDeployer{
		client:  client,
		applier: applier,
	}
}

func (c *crdDeployer) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(r)), kubernetes.DefaultMergeFuncs)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (c *crdDeployer) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(r))))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// Wait signals whether a CRD is ready or needs more time to be deployed.
func (c *crdDeployer) Wait(ctx context.Context) error {
	return kubernetesutils.WaitUntilCRDManifestsReady(ctx, c.client, resources)
}

// WaitCleanup for destruction to finish and component to be fully removed. crdDeployer does not need to wait for cleanup.
func (c *crdDeployer) WaitCleanup(_ context.Context) error {
	return nil
}
