// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
)

var (
	//go:embed assets/crd-opentelemetry.io_opentelemetrycollectors.yaml
	openTelemetryOpenTelemetryCollectorCRD string
	//go:embed assets/crd-opentelemetry.io_instrumentations.yaml
	openTelemetryInstrumentationCRD string
	//go:embed assets/crd-opentelemetry.io_opampbridges.yaml
	openTelemetryOpAMPBridgeCRD string
	//go:embed assets/crd-opentelemetry.io_targetallocators.yaml
	openTelemetryTargetAllocatorCRD string
)

// NewCRDs can be used to deploy OpenTelemetry Operator CRDs
func NewCRDs(client client.Client) (component.DeployWaiter, error) {
	resources := []string{
		openTelemetryOpenTelemetryCollectorCRD,
		openTelemetryInstrumentationCRD,
		openTelemetryOpAMPBridgeCRD,
		openTelemetryTargetAllocatorCRD,
	}

	return crddeployer.New(client, resources, false)
}
