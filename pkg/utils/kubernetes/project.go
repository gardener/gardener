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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencore "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

func tryUpdateProject(
	g gardencore.Interface,
	backoff wait.Backoff,
	meta metav1.ObjectMeta,
	transform func(*gardencorev1beta1.Project) (*gardencorev1beta1.Project, error),
	updateFunc func(g gardencore.Interface, project *gardencorev1beta1.Project) (*gardencorev1beta1.Project, error),
	compare func(cur, updated *gardencorev1beta1.Project) bool,
) (*gardencorev1beta1.Project, error) {
	var (
		result  *gardencorev1beta1.Project
		attempt int
	)

	err := retry.RetryOnConflict(backoff, func() (err error) {
		attempt++
		cur, err := g.CoreV1beta1().Projects().Get(meta.Name, metav1.GetOptions{})
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
			logger.Logger.Errorf("Attempt %d failed to update Project %s due to %v", attempt, cur.Name, err)
		}
		return
	})
	if err != nil {
		logger.Logger.Errorf("Failed to updated Project %s after %d attempts due to %v", meta.Name, attempt, err)
	}

	return result, err
}

// TryUpdateProject tries to update a project and retries the operation with the given <backoff>.
func TryUpdateProject(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.Project) (*gardencorev1beta1.Project, error)) (*gardencorev1beta1.Project, error) {
	return tryUpdateProject(g, backoff, meta, transform, func(g gardencore.Interface, project *gardencorev1beta1.Project) (*gardencorev1beta1.Project, error) {
		return g.CoreV1beta1().Projects().Update(project)
	}, func(cur, updated *gardencorev1beta1.Project) bool {
		return equality.Semantic.DeepEqual(cur, updated)
	})
}

// TryUpdateProjectStatus tries to update a project's status and retries the operation with the given <backoff>.
func TryUpdateProjectStatus(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.Project) (*gardencorev1beta1.Project, error)) (*gardencorev1beta1.Project, error) {
	return tryUpdateProject(g, backoff, meta, transform, func(g gardencore.Interface, project *gardencorev1beta1.Project) (*gardencorev1beta1.Project, error) {
		return g.CoreV1beta1().Projects().UpdateStatus(project)
	}, func(cur, updated *gardencorev1beta1.Project) bool {
		return equality.Semantic.DeepEqual(cur.Status, updated.Status)
	})
}
