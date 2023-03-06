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

package shared

import (
	"time"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewGardenerResourceManager instantiates a new `gardener-resource-manager` component.
func NewGardenerResourceManager(
	c client.Client,
	gardenNamespaceName string,
	runtimeVersion *semver.Version,
	imageVector imagevector.ImageVector,
	secretsManager secretsmanager.Interface,
	logLevel, logFormat string,
	secretNameServerCA string,
	priorityClassName string,
	defaultSeccompProfileEnabled bool,
	endpointSliceHintsEnabled bool,
	zones []string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imageVector.FindImage(images.ImageNameGardenerResourceManager)
	if err != nil {
		return nil, err
	}

	repository, tag := image.String(), version.Get().GitVersion
	if image.Tag != nil {
		repository, tag = image.Repository, *image.Tag
	}
	image = &imagevector.Image{Repository: repository, Tag: &tag}

	return resourcemanager.New(c, gardenNamespaceName, secretsManager, resourcemanager.Values{
		ConcurrentSyncs:                      pointer.Int(20),
		DefaultSeccompProfileEnabled:         defaultSeccompProfileEnabled,
		EndpointSliceHintsEnabled:            endpointSliceHintsEnabled,
		HealthSyncPeriod:                     &metav1.Duration{Duration: time.Minute},
		Image:                                image.String(),
		LogLevel:                             logLevel,
		LogFormat:                            logFormat,
		MaxConcurrentTokenInvalidatorWorkers: pointer.Int(5),
		// TODO(timuthy): Remove PodTopologySpreadConstraints webhook once for all seeds the
		//  MatchLabelKeysInPodTopologySpread feature gate is beta and enabled by default (probably 1.26+).
		PodTopologySpreadConstraintsEnabled: true,
		PriorityClassName:                   priorityClassName,
		Replicas:                            pointer.Int32(2),
		ResourceClass:                       pointer.String(v1beta1constants.SeedResourceManagerClass),
		SecretNameServerCA:                  secretNameServerCA,
		SyncPeriod:                          &metav1.Duration{Duration: time.Hour},
		KubernetesVersion:                   runtimeVersion,
		VPA: &resourcemanager.VPAConfig{
			MinAllowed: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
		Zones: zones,
	}), nil
}
