// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"fmt"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/version"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
)

// RespectShootSyncPeriodOverwrite checks whether to respect the sync period overwrite of a Shoot or not.
func RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite bool, shoot *v1beta1.Shoot) bool {
	return respectSyncPeriodOverwrite || shoot.Namespace == constants.GardenNamespace
}

// ShouldIgnoreShoot determines whether a Shoot should be ignored or not.
func ShouldIgnoreShoot(respectSyncPeriodOverwrite bool, shoot *v1beta1.Shoot) bool {
	if !RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite, shoot) {
		return false
	}

	value, ok := shoot.Annotations[constants.ShootIgnore]
	if !ok {
		return false
	}

	ignore, _ := strconv.ParseBool(value)
	return ignore
}

// IsShootFailed checks if a Shoot is failed.
func IsShootFailed(shoot *v1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation

	return lastOperation != nil && lastOperation.State == v1beta1.LastOperationStateFailed &&
		shoot.Generation == shoot.Status.ObservedGeneration &&
		shoot.Status.Gardener.Version == version.Get().GitVersion
}

// IsNowInEffectiveShootMaintenanceTimeWindow checks if the current time is in the effective
// maintenance time window of the Shoot.
func IsNowInEffectiveShootMaintenanceTimeWindow(shoot *v1beta1.Shoot) bool {
	return EffectiveShootMaintenanceTimeWindow(shoot).Contains(time.Now())
}

// LastReconciliationDuringThisTimeWindow returns true if <now> is contained in the given effective maintenance time
// window of the shoot and if the <lastReconciliation> did not happen longer than the longest possible duration of a
// maintenance time window.
func LastReconciliationDuringThisTimeWindow(shoot *v1beta1.Shoot) bool {
	if shoot.Status.LastOperation == nil {
		return false
	}

	var (
		timeWindow         = EffectiveShootMaintenanceTimeWindow(shoot)
		now                = time.Now()
		lastReconciliation = shoot.Status.LastOperation.LastUpdateTime.Time
	)

	return timeWindow.Contains(lastReconciliation) && now.UTC().Sub(lastReconciliation.UTC()) <= v1beta1.MaintenanceTimeWindowDurationMaximum
}

// IsObservedAtLatestGenerationAndSucceeded checks whether the Shoot's generation has changed or if the LastOperation status
// is Succeeded.
func IsObservedAtLatestGenerationAndSucceeded(shoot *v1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration &&
		(lastOperation != nil && lastOperation.State == v1beta1.LastOperationStateSucceeded)
}

// SyncPeriodOfShoot determines the sync period of the given shoot.
//
// If no overwrite is allowed, the defaultMinSyncPeriod is returned.
// Otherwise, the overwrite is parsed. If an error occurs or it is smaller than the defaultMinSyncPeriod,
// the defaultMinSyncPeriod is returned. Otherwise, the overwrite is returned.
func SyncPeriodOfShoot(respectSyncPeriodOverwrite bool, defaultMinSyncPeriod time.Duration, shoot *v1beta1.Shoot) time.Duration {
	if !RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite, shoot) {
		return defaultMinSyncPeriod
	}

	syncPeriodOverwrite, ok := shoot.Annotations[constants.ShootSyncPeriod]
	if !ok {
		return defaultMinSyncPeriod
	}

	syncPeriod, err := time.ParseDuration(syncPeriodOverwrite)
	if err != nil {
		return defaultMinSyncPeriod
	}

	if syncPeriod < defaultMinSyncPeriod {
		return defaultMinSyncPeriod
	}
	return syncPeriod
}

// EffectiveMaintenanceTimeWindow cuts a maintenance time window at the end with a guess of 15 minutes. It is subtracted from the end
// of a maintenance time window to use a best-effort kind of finishing the operation before the end.
// Generally, we can't make sure that the maintenance operation is done by the end of the time window anyway (considering large
// clusters with hundreds of nodes, a rolling update will take several hours).
func EffectiveMaintenanceTimeWindow(timeWindow *utils.MaintenanceTimeWindow) *utils.MaintenanceTimeWindow {
	return timeWindow.WithEnd(timeWindow.End().Add(0, -15, 0))
}

// EffectiveShootMaintenanceTimeWindow returns the effective MaintenanceTimeWindow of the given Shoot.
func EffectiveShootMaintenanceTimeWindow(shoot *v1beta1.Shoot) *utils.MaintenanceTimeWindow {
	maintenance := shoot.Spec.Maintenance
	if maintenance == nil || maintenance.TimeWindow == nil {
		return utils.AlwaysTimeWindow
	}

	timeWindow, err := utils.ParseMaintenanceTimeWindow(maintenance.TimeWindow.Begin, maintenance.TimeWindow.End)
	if err != nil {
		return utils.AlwaysTimeWindow
	}

	return EffectiveMaintenanceTimeWindow(timeWindow)
}

// GardenEtcdEncryptionSecretName returns the name to the 'backup' of the etcd encryption secret in the Garden cluster.
func GardenEtcdEncryptionSecretName(shootName string) string {
	return fmt.Sprintf("%s.%s", shootName, common.EtcdEncryptionSecretName)
}

// GetShootNameFromOwnerReferences attempts to get the name of the Shoot object which owns the passed in object.
// If it is not owned by a Shoot, an empty string is returned.
func GetShootNameFromOwnerReferences(objectMeta metav1.Object) string {
	for _, ownerRef := range objectMeta.GetOwnerReferences() {
		if ownerRef.Kind == "Shoot" {
			return ownerRef.Name
		}
	}
	return ""
}
