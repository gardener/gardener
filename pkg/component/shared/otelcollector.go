// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	"github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector"
)

// NewOtelCollector returns new OtelCollector deployer
func NewOtelCollector(
	c client.Client,
	namespace string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameOpentelemetryCollectorControlPlane)
	if err != nil {
		return nil, err
	}

	deployer := collector.New(
		c,
		namespace,
		collector.Values{
			Image: image.String(),
		},
		"http://"+constants.ServiceName+":"+strconv.Itoa(constants.ValiPort)+constants.PushEndpoint,
	)

	return deployer, nil
}
