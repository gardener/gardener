// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentoperator

import (
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
)

var (
	//go:embed assets/crd-fluentbit.fluent.io_clusterfilters.yaml
	fluentBitClusterFilterCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusterfluentbitconfigs.yaml
	fluentBitClusterFBConfigCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusterinputs.yaml
	fluentBitClusterInputCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusteroutputs.yaml
	fluentBitClusterOutputCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clusterparsers.yaml
	fluentBitClusterParserCRD string
	//go:embed assets/crd-fluentbit.fluent.io_fluentbits.yaml
	fluentBitCRD string
	//go:embed assets/crd-fluentbit.fluent.io_collectors.yaml
	fluentBitCollectorCRD string
	//go:embed assets/crd-fluentbit.fluent.io_fluentbitconfigs.yaml
	fluentBitConfigCRD string
	//go:embed assets/crd-fluentbit.fluent.io_filters.yaml
	fluentBitFilterCRD string
	//go:embed assets/crd-fluentbit.fluent.io_parsers.yaml
	fluentBitParserCRD string
	//go:embed assets/crd-fluentbit.fluent.io_outputs.yaml
	fluentBitOutputCRD string
	//go:embed assets/crd-fluentbit.fluent.io_clustermultilineparsers.yaml
	fluentBitClusterMultilineParserCRD string
	//go:embed assets/crd-fluentbit.fluent.io_multilineparsers.yaml
	fluentBitMultilineParserCRD string
)

// NewCRDs can be used to deploy Fluent Operator CRDs.
func NewCRDs(client client.Client, applier kubernetes.Applier) (component.DeployWaiter, error) {
	resources := []string{
		fluentBitClusterFilterCRD,
		fluentBitClusterFBConfigCRD,
		fluentBitClusterInputCRD,
		fluentBitClusterOutputCRD,
		fluentBitClusterParserCRD,
		fluentBitCRD,
		fluentBitCollectorCRD,
		fluentBitConfigCRD,
		fluentBitFilterCRD,
		fluentBitParserCRD,
		fluentBitOutputCRD,
		fluentBitClusterMultilineParserCRD,
		fluentBitMultilineParserCRD,
	}
	return crddeployer.New(client, applier, resources, false)
}
