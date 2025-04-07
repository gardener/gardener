// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentcustomresources"
)

// NewFluentOperatorCustomResources instantiates a new `Fluent Operator Custom Resources` component.
func NewFluentOperatorCustomResources(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	suffix string,
	centralLoggingConfigurations []component.CentralLoggingConfiguration,
	output *fluentbitv1alpha2.ClusterOutput,
) (
	deployer component.DeployWaiter,
	err error,
) {
	var (
		inputs  []*fluentbitv1alpha2.ClusterInput
		filters []*fluentbitv1alpha2.ClusterFilter
		parsers []*fluentbitv1alpha2.ClusterParser
		outputs []*fluentbitv1alpha2.ClusterOutput
	)

	// Fetch component specific logging configurations
	for _, componentFn := range centralLoggingConfigurations {
		loggingConfig, err := componentFn()
		if err != nil {
			return nil, err
		}

		if len(loggingConfig.Inputs) > 0 {
			inputs = append(inputs, loggingConfig.Inputs...)
		}

		if len(loggingConfig.Filters) > 0 {
			filters = append(filters, loggingConfig.Filters...)
		}

		if len(loggingConfig.Parsers) > 0 {
			parsers = append(parsers, loggingConfig.Parsers...)
		}
	}

	if output != nil {
		outputs = append(outputs, output)
	}

	deployer = fluentcustomresources.New(
		c,
		gardenNamespaceName,
		fluentcustomresources.Values{
			Suffix:  suffix,
			Inputs:  inputs,
			Filters: filters,
			Parsers: parsers,
			Outputs: outputs,
		},
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}
