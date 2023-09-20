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
	"net"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
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
	hvpaConfig *kubecontrollermanager.HVPAConfig,
	podNetwork *net.IPNet,
	serviceNetwork *net.IPNet,
	clusterSigningDuration *time.Duration,
	controllerWorkers kubecontrollermanager.ControllerWorkers,
	controllerSyncPeriods kubecontrollermanager.ControllerSyncPeriods,
) (
	kubecontrollermanager.Interface,
	error,
) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameKubeControllerManager, imagevectorutils.RuntimeVersion(runtimeVersion.String()), imagevectorutils.TargetVersion(targetVersion.String()))
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
			HVPAConfig:             hvpaConfig,
			IsWorkerless:           isWorkerless,
			PodNetwork:             podNetwork,
			ServiceNetwork:         serviceNetwork,
			ClusterSigningDuration: clusterSigningDuration,
			ControllerWorkers:      controllerWorkers,
			ControllerSyncPeriods:  controllerSyncPeriods,
		},
	), nil
}
