// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	garden "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

func tryUpdateBackupInfrastructure(
	g garden.Interface,
	backoff wait.Backoff,
	meta metav1.ObjectMeta,
	transform func(*gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error),
	updateFunc func(g garden.Interface, backupInfrastructure *gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error),
	compare func(cur, updated *gardenv1beta1.BackupInfrastructure) bool,
) (*gardenv1beta1.BackupInfrastructure, error) {
	var (
		result  *gardenv1beta1.BackupInfrastructure
		attempt int
	)

	err := retry.RetryOnConflict(backoff, func() (err error) {
		attempt++
		cur, err := g.GardenV1beta1().BackupInfrastructures(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		updated, err := transform(cur.DeepCopy())
		if err != nil {
			return err
		}

		if compare(cur, updated) {
			result = cur
			return nil
		}

		result, err = updateFunc(g, updated)
		if err != nil {
			logger.Logger.Errorf("Attempt %d failed to update BackupInfrastructure %s due to %v", attempt, cur.Name, err)
		}
		return
	})
	if err != nil {
		logger.Logger.Errorf("Failed to updated BackupInfrastructure %s after %d attempts due to %v", meta.Name, attempt, err)
	}

	return result, err
}

// TryUpdateBackupInfrastructureStatus tries to update a BackupInfrastructure's status and retries the operation with the given <backoff>.
func TryUpdateBackupInfrastructureStatus(g garden.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error)) (*gardenv1beta1.BackupInfrastructure, error) {
	return tryUpdateBackupInfrastructure(g, backoff, meta, transform, func(g garden.Interface, backupInfrastructure *gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error) {
		return g.GardenV1beta1().BackupInfrastructures(meta.Namespace).UpdateStatus(backupInfrastructure)
	}, func(cur, updated *gardenv1beta1.BackupInfrastructure) bool {
		return equality.Semantic.DeepEqual(cur.Status, updated.Status)
	})
}

// TryUpdateBackupInfrastructureAnnotations tries to update a BackupInfrastructure's annotations and retries the operation with the given <backoff>.
func TryUpdateBackupInfrastructureAnnotations(g garden.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error)) (*gardenv1beta1.BackupInfrastructure, error) {
	return tryUpdateBackupInfrastructure(g, backoff, meta, transform, func(g garden.Interface, backupInfrastructure *gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error) {
		return g.GardenV1beta1().BackupInfrastructures(meta.Namespace).Update(backupInfrastructure)
	}, func(cur, updated *gardenv1beta1.BackupInfrastructure) bool {
		return equality.Semantic.DeepEqual(cur.Annotations, updated.Annotations)
	})
}
