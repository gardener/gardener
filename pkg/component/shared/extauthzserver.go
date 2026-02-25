// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/networking/extauthzserver"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewExtAuthzServer instantiates a new `ext-authz-server` component.
func NewExtAuthzServer(
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
	extAuthzServerImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameExtAuthzServer)
	if err != nil {
		return nil, err
	}

	deployer = extauthzserver.New(
		c,
		namespace,
		secretsManager,
		extauthzserver.Values{
			Image:             extAuthzServerImage.String(),
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
