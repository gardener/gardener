// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/vali"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewVali returns new Vali deployer
func NewVali(
	c client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	clusterType component.ClusterType,
	replicas int32,
	isShootNodeLoggingEnabled bool,
	priorityClassName string,
	storage *resource.Quantity,
	ingressHost string,
) (
	vali.Interface,
	error,
) {
	valiImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVali)
	if err != nil {
		return nil, err
	}

	curatorImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameValiCurator)
	if err != nil {
		return nil, err
	}

	tune2fsImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameTune2fs)
	if err != nil {
		return nil, err
	}

	kubeRBACProxyImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameKubeRbacProxy)
	if err != nil {
		return nil, err
	}

	telegrafImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameTelegraf)
	if err != nil {
		return nil, err
	}

	deployer := vali.New(c, namespace, secretsManager, vali.Values{
		ValiImage:               valiImage.String(),
		CuratorImage:            curatorImage.String(),
		InitLargeDirImage:       tune2fsImage.String(),
		KubeRBACProxyImage:      kubeRBACProxyImage.String(),
		TelegrafImage:           telegrafImage.String(),
		Replicas:                replicas,
		ShootNodeLoggingEnabled: isShootNodeLoggingEnabled,
		PriorityClassName:       priorityClassName,
		Storage:                 storage,
		ClusterType:             clusterType,
		IngressHost:             ingressHost,
	})

	return deployer, nil
}
