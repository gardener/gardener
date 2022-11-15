// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardensystem"
	sharedcomponent "github.com/gardener/gardener/pkg/operation/botanist/component/shared"
	operatorfeatures "github.com/gardener/gardener/pkg/operator/features"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (r *Reconciler) newGardenerResourceManager(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	return sharedcomponent.NewGardenerResourceManager(
		r.RuntimeClient,
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		secretsManager,
		r.Config.LogLevel, r.Config.LogFormat,
		operatorv1alpha1.SecretNameCARuntime,
		v1beta1constants.PriorityClassNameGardenSystemCritical,
		operatorfeatures.FeatureGate.Enabled(features.DefaultSeccompProfile),
		garden.Spec.RuntimeCluster.Provider.Zones,
	)
}

func (r *Reconciler) newVerticalPodAutoscaler(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	return sharedcomponent.NewVerticalPodAutoscaler(
		r.RuntimeClient,
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		secretsManager,
		vpaEnabled(garden.Spec.RuntimeCluster.Settings),
		operatorv1alpha1.SecretNameCARuntime,
		v1beta1constants.PriorityClassNameGardenSystem300,
		v1beta1constants.PriorityClassNameGardenSystem200,
		v1beta1constants.PriorityClassNameGardenSystem200,
	)
}

func (r *Reconciler) newHVPA() (component.DeployWaiter, error) {
	return sharedcomponent.NewHVPA(
		r.RuntimeClient,
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		hvpaEnabled(),
		v1beta1constants.PriorityClassNameGardenSystem200,
	)
}

func (r *Reconciler) newSystem() component.DeployWaiter {
	return gardensystem.New(r.RuntimeClient, r.GardenNamespace)
}
