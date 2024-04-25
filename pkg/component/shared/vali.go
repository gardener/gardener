// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
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
	hvpaEnabled bool,
	maintenanceTimeWindow *hvpav1alpha1.MaintenanceTimeWindow,
) (
	vali.Interface,
	error,
) {
	valiImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameVali)
	if err != nil {
		return nil, err
	}

	curatorImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameValiCurator)
	if err != nil {
		return nil, err
	}

	tune2fsImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameTune2fs)
	if err != nil {
		return nil, err
	}

	kubeRBACProxyImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameKubeRbacProxy)
	if err != nil {
		return nil, err
	}

	telegrafImage, err := imagevector.ImageVector().FindImage(imagevector.ImageNameTelegraf)
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
		HVPAEnabled:             hvpaEnabled,
		MaintenanceTimeWindow:   maintenanceTimeWindow,
		ShootNodeLoggingEnabled: isShootNodeLoggingEnabled,
		PriorityClassName:       priorityClassName,
		Storage:                 storage,
		ClusterType:             clusterType,
		IngressHost:             ingressHost,
	})

	return deployer, nil
}
