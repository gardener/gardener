// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewKubeControllerManager returns a deployer for the kube-controller-manager.
func NewKubeControllerManager(
	log logr.Logger,
	runtimeClientSet kubernetes.Interface,
	runtimeNamespace string,
	runtimeVersion *semver.Version,
	targetVersion *semver.Version,
	secretsManager secretsmanager.Interface,
	namePrefix string,
	config *gardencorev1beta1.KubeControllerManagerConfig,
	priorityClassName string,
	isWorkerless bool,
	isScaleDownDisabled bool,
	clusterSigningDuration *time.Duration,
	controllerWorkers kubecontrollermanager.ControllerWorkers,
	controllerSyncPeriods kubecontrollermanager.ControllerSyncPeriods,
	managedResourceLabels map[string]string,
) (
	kubecontrollermanager.Interface,
	error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameKubeControllerManager, imagevectorutils.RuntimeVersion(runtimeVersion.String()), imagevectorutils.TargetVersion(targetVersion.String()))
	if err != nil {
		return nil, err
	}

	return kubecontrollermanager.New(
		log.WithValues("component", "kube-controller-manager"),
		runtimeClientSet,
		runtimeNamespace,
		secretsManager,
		kubecontrollermanager.Values{
			RuntimeVersion:         runtimeVersion,
			TargetVersion:          targetVersion,
			Image:                  image.String(),
			Config:                 config,
			PriorityClassName:      priorityClassName,
			NamePrefix:             namePrefix,
			IsScaleDownDisabled:    isScaleDownDisabled,
			IsWorkerless:           isWorkerless,
			ClusterSigningDuration: clusterSigningDuration,
			ControllerWorkers:      controllerWorkers,
			ControllerSyncPeriods:  controllerSyncPeriods,
			ManagedResourceLabels:  managedResourceLabels,
		},
	), nil
}
