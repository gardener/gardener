// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared

import (
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/logging/vali"
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
