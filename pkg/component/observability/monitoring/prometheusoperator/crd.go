// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheusoperator

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
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
)

// NewCRDs can be used to deploy the CRD definitions for the Prometheus Operator.
func NewCRDs(client client.Client, applier kubernetes.Applier) (component.DeployWaiter, error) {
	resources := []string{
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
	}
	return crddeployer.New(client, applier, resources, false)
}
