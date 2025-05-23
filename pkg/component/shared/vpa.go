// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"time"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewVerticalPodAutoscaler instantiates a new `vertical-pod-autoscaler` component.
func NewVerticalPodAutoscaler(
	c client.Client,
	gardenNamespaceName string,
	runtimeVersion *semver.Version,
	secretsManager secretsmanager.Interface,
	enabled bool,
	secretNameServerCA string,
	priorityClassNameAdmissionController string,
	priorityClassNameRecommender string,
	priorityClassNameUpdater string,
	isGardenCluster bool,
) (
	component.DeployWaiter,
	error,
) {
	imageAdmissionController, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVpaAdmissionController, imagevectorutils.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}

	imageRecommender, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVpaRecommender, imagevectorutils.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}

	imageUpdater, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVpaUpdater, imagevectorutils.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}

	return vpa.New(
		c,
		gardenNamespaceName,
		secretsManager,
		vpa.Values{
			ClusterType:              component.ClusterTypeSeed,
			IsGardenCluster:          isGardenCluster,
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
				RecommendationMarginFraction: ptr.To(float64(0.05)),
			},
			Updater: vpa.ValuesUpdater{
				EvictionTolerance:      ptr.To(float64(1.0)),
				EvictAfterOOMThreshold: &metav1.Duration{Duration: 48 * time.Hour},
				Image:                  imageUpdater.String(),
				PriorityClassName:      priorityClassNameUpdater,
			},
		},
	), nil
}
