// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetryoperator

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
)

var (
	//go:embed assets/crd-opentelemetry.io_opentelemetrycollectors.yaml
	openTelemetryOpenTelemetryCollectorCRD string
	//go:embed assets/crd-opentelemetry.io_instrumentations.yaml
	openTelemetryInstrumentationCRD string
	//go:embed assets/crd-opentelemetry.io_opampbridges.yaml
	openTelemetryOpenTelemetryCollectorBridgeCRD string
	//go:embed assets/crd-opentelemetry.io_targetallocators.yaml
	openTelemetryOpenTelemetryTargetAllocatorCRD string
)

// NewCRDs can be used to deploy OpenTelemetry Operator CRDS
func NewCRDs(client client.Client, applier kubernetes.Applier) (component.DeployWaiter, error) {
	resources := []string{
		openTelemetryOpenTelemetryCollectorCRD,
		openTelemetryInstrumentationCRD,
		openTelemetryOpenTelemetryCollectorBridgeCRD,
		openTelemetryOpenTelemetryTargetAllocatorCRD,
	}

	return crddeployer.New(client, applier, resources, false)
}
