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
	"github.com/Masterminds/semver"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewVali returns new Vali deployer
func NewVali(
	c client.Client,
	imageVector imagevector.ImageVector,
	replicas int32,
	namespace string,
	ingressClass string,
	priorityClassName string,
	clusterType component.ClusterType,
	valiHost string,
	secretsManager secretsmanager.Interface,
	storage *resource.Quantity,
	k8Version *semver.Version,
	isLoggingEnabled bool,
	isShootNodeLoggingEnabled bool,
	authEnabled bool,
	hvpaEnabled bool,
	maintenanceTimeWindow *hvpav1alpha1.MaintenanceTimeWindow,
) (
	component.Deployer,
	error,
) {
	valiImage, err := imageVector.FindImage(images.ImageNameVali)
	if err != nil {
		return nil, err
	}

	curatorImage, err := imageVector.FindImage(images.ImageNameValiCurator)
	if err != nil {
		return nil, err
	}

	tune2fsImage, err := imageVector.FindImage(images.ImageNameTune2fs)
	if err != nil {
		return nil, err
	}

	alpineImage, err := imageVector.FindImage(images.ImageNameAlpine)
	if err != nil {
		return nil, err
	}

	kubeRBACProxyImage, err := imageVector.FindImage(images.ImageNameKubeRbacProxy)
	if err != nil {
		return nil, err
	}

	telegrafImage, err := imageVector.FindImage(images.ImageNameTelegraf)
	if err != nil {
		return nil, err
	}

	deployer := vali.New(c, namespace, secretsManager, k8Version, vali.Values{
		Replicas:              replicas,
		HvpaEnabled:           hvpaEnabled,
		MaintenanceTimeWindow: maintenanceTimeWindow,
		RBACProxyEnabled:      isShootNodeLoggingEnabled,
		PriorityClassName:     priorityClassName,
		Storage:               storage,
		AuthEnabled:           authEnabled,
		ClusterType:           clusterType,
		IngressClass:          ingressClass,
		ValiHost:              valiHost,
		ValiImage:             valiImage.String(),
		CuratorImage:          curatorImage.String(),
		RenameLokiToValiImage: alpineImage.String(),
		InitLargeDirImage:     tune2fsImage.String(),
		KubeRBACProxyImage:    kubeRBACProxyImage.String(),
		TelegrafImage:         telegrafImage.String(),
	})

	if !isLoggingEnabled {
		return component.OpDestroy(deployer), nil
	}

	return deployer, err
}
