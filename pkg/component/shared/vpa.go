// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	"github.com/Masterminds/semver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewVerticalPodAutoscaler instantiates a new `vertical-pod-autoscaler` component.
func NewVerticalPodAutoscaler(
	c client.Client,
	gardenNamespaceName string,
	runtimeVersion *semver.Version,
	imageVector imagevector.ImageVector,
	secretsManager secretsmanager.Interface,
	enabled bool,
	secretNameServerCA string,
	priorityClassNameAdmissionController string,
	priorityClassNameRecommender string,
	priorityClassNameUpdater string,
) (
	component.DeployWaiter,
	error,
) {
	imageAdmissionController, err := imageVector.FindImage(images.ImageNameVpaAdmissionController, imagevector.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}

	imageRecommender, err := imageVector.FindImage(images.ImageNameVpaRecommender, imagevector.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}

	imageUpdater, err := imageVector.FindImage(images.ImageNameVpaUpdater, imagevector.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}

	return vpa.New(
		c,
		gardenNamespaceName,
		secretsManager,
		vpa.Values{
			ClusterType:              component.ClusterTypeSeed,
			Enabled:                  enabled,
			SecretNameServerCA:       secretNameServerCA,
			RuntimeKubernetesVersion: runtimeVersion,
			AdmissionController: vpa.ValuesAdmissionController{
				Image:             imageAdmissionController.String(),
				PriorityClassName: priorityClassNameAdmissionController,
			},
			Recommender: vpa.ValuesRecommender{
				Image:                        imageRecommender.String(),
				PriorityClassName:            priorityClassNameRecommender,
				RecommendationMarginFraction: pointer.Float64(0.05),
			},
			Updater: vpa.ValuesUpdater{
				EvictionTolerance:      pointer.Float64(1.0),
				EvictAfterOOMThreshold: &metav1.Duration{Duration: 48 * time.Hour},
				Image:                  imageUpdater.String(),
				PriorityClassName:      priorityClassNameUpdater,
			},
		},
	), nil
}
