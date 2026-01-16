// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	"github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewOpenTelemetryCollector instantiates a new `OpenTelemetryOperator` component.
func NewOpenTelemetryCollector(
	c client.Client,
	gardenNamespaceName string,
	priorityClassName string,
	secretsManager secretsmanager.Interface,
	secretNameServerCA string,
) (
	deployer collector.Interface,
	err error,
) {
	collectorImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameOpentelemetryCollector)
	if err != nil {
		return nil, err
	}

	kubeRBACProxyImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameKubeRbacProxy)
	if err != nil {
		return nil, err
	}

	return collector.New(
		c,
		gardenNamespaceName,
		collector.Values{
			Image:                   collectorImage.String(),
			KubeRBACProxyImage:      kubeRBACProxyImage.String(),
			LokiEndpoint:            "http://" + valiconstants.ServiceName + ":" + strconv.Itoa(valiconstants.ValiPort) + valiconstants.PushEndpoint,
			Replicas:                1,
			ShootNodeLoggingEnabled: false,
			IngressHost:             "",
			SecretNameServerCA:      secretNameServerCA,
			PriorityClassName:       priorityClassName,
		},
		secretsManager,
	), nil
}
