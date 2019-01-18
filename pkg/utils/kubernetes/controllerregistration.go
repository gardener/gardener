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
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencore "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

func tryUpdateControllerRegistration(
	g gardencore.Interface,
	backoff wait.Backoff,
	meta metav1.ObjectMeta,
	transform func(*gardencorev1alpha1.ControllerRegistration) (*gardencorev1alpha1.ControllerRegistration, error),
	updateFunc func(g gardencore.Interface, controllerRegistration *gardencorev1alpha1.ControllerRegistration) (*gardencorev1alpha1.ControllerRegistration, error),
	equalFunc func(cur, updated *gardencorev1alpha1.ControllerRegistration) bool,
) (*gardencorev1alpha1.ControllerRegistration, error) {

	var (
		result  *gardencorev1alpha1.ControllerRegistration
		attempt int
	)
	err := retry.RetryOnConflict(backoff, func() (err error) {
		attempt++
		cur, err := g.CoreV1alpha1().ControllerRegistrations().Get(meta.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		updated, err := transform(cur.DeepCopy())
		if err != nil {
			return err
		}

		if equalFunc(cur, updated) {
			result = cur
			return nil
		}

		result, err = updateFunc(g, updated)
		if err != nil {
			logger.Logger.Errorf("Attempt %d failed to update ControllerRegistration %s due to %v", attempt, cur.Name, err)
		}
		return
	})
	if err != nil {
		logger.Logger.Errorf("Failed to updated ControllerRegistration %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return result, err
}

// TryUpdateControllerRegistrationWithEqualFunc tries to update the status of the controllerRegistration matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerRegistration object. If the equal
// func concludes a semantically equal ControllerRegistration, no update is done and the operation returns normally.
func TryUpdateControllerRegistrationWithEqualFunc(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.ControllerRegistration) (*gardencorev1alpha1.ControllerRegistration, error), equal func(cur, updated *gardencorev1alpha1.ControllerRegistration) bool) (*gardencorev1alpha1.ControllerRegistration, error) {
	return tryUpdateControllerRegistration(g, backoff, meta, transform, func(g gardencore.Interface, controllerRegistration *gardencorev1alpha1.ControllerRegistration) (*gardencorev1alpha1.ControllerRegistration, error) {
		return g.CoreV1alpha1().ControllerRegistrations().Update(controllerRegistration)
	}, equal)
}

// TryUpdateControllerRegistration tries to update the status of the controllerRegistration matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerRegistration object. If the transformation
// yields a semantically equal ControllerRegistration, no update is done and the operation returns normally.
func TryUpdateControllerRegistration(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.ControllerRegistration) (*gardencorev1alpha1.ControllerRegistration, error)) (*gardencorev1alpha1.ControllerRegistration, error) {
	return TryUpdateControllerRegistrationWithEqualFunc(g, backoff, meta, transform, func(cur, updated *gardencorev1alpha1.ControllerRegistration) bool {
		return equality.Semantic.DeepEqual(cur, updated)
	})
}
