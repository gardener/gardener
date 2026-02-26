// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/networking/istiobasicauthserver"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewIstioBasicAuthServer instantiates a new `istio-basic-auth-server` component.
func NewIstioBasicAuthServer(
	c client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	enabled bool,
	replicas int32,
	priorityClassName string,
	isGardenCluster bool,
) (
	deployer component.DeployWaiter,
	err error,
) {
	istioBasicAuthServerImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameIstioBasicAuthServer)
	if err != nil {
		return nil, err
	}

	deployer = istiobasicauthserver.New(
		c,
		namespace,
		secretsManager,
		istiobasicauthserver.Values{
			Image:             istioBasicAuthServerImage.String(),
			PriorityClassName: priorityClassName,
			Replicas:          replicas,
			IsGardenCluster:   isGardenCluster,
		},
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}
