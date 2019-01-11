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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

func tryUpdateControllerInstallation(
	g gardencore.Interface,
	backoff wait.Backoff,
	meta metav1.ObjectMeta,
	transform func(*gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error),
	updateFunc func(g gardencore.Interface, controllerInstallation *gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error),
	equalFunc func(cur, updated *gardencorev1alpha1.ControllerInstallation) bool,
) (*gardencorev1alpha1.ControllerInstallation, error) {

	var (
		result  *gardencorev1alpha1.ControllerInstallation
		attempt int
	)
	err := retry.RetryOnConflict(backoff, func() (err error) {
		attempt++
		cur, err := g.CoreV1alpha1().ControllerInstallations().Get(meta.Name, metav1.GetOptions{})
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
			logger.Logger.Errorf("Attempt %d failed to update ControllerInstallation %s due to %v", attempt, cur.Name, err)
		}
		return
	})
	if err != nil {
		logger.Logger.Errorf("Failed to updated ControllerInstallation %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return result, err
}

// TryUpdateControllerInstallationWithEqualFunc tries to update the status of the controllerInstallation matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerInstallation object. If the equal
// func concludes a semantically equal ControllerInstallation, no update is done and the operation returns normally.
func TryUpdateControllerInstallationWithEqualFunc(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error), equal func(cur, updated *gardencorev1alpha1.ControllerInstallation) bool) (*gardencorev1alpha1.ControllerInstallation, error) {
	return tryUpdateControllerInstallation(g, backoff, meta, transform, func(g gardencore.Interface, controllerInstallation *gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error) {
		return g.CoreV1alpha1().ControllerInstallations().Update(controllerInstallation)
	}, equal)
}

// TryUpdateControllerInstallation tries to update the status of the controllerInstallation matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerInstallation object. If the transformation
// yields a semantically equal ControllerInstallation, no update is done and the operation returns normally.
func TryUpdateControllerInstallation(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error)) (*gardencorev1alpha1.ControllerInstallation, error) {
	return TryUpdateControllerInstallationWithEqualFunc(g, backoff, meta, transform, func(cur, updated *gardencorev1alpha1.ControllerInstallation) bool {
		return equality.Semantic.DeepEqual(cur, updated)
	})
}

// TryUpdateControllerInstallationStatusWithEqualFunc tries to update the status of the controllerInstallation matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerInstallation object. If the equal
// func concludes a semantically equal ControllerInstallation, no update is done and the operation returns normally.
func TryUpdateControllerInstallationStatusWithEqualFunc(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error), equal func(cur, updated *gardencorev1alpha1.ControllerInstallation) bool) (*gardencorev1alpha1.ControllerInstallation, error) {
	return tryUpdateControllerInstallation(g, backoff, meta, transform, func(g gardencore.Interface, controllerInstallation *gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error) {
		return g.CoreV1alpha1().ControllerInstallations().UpdateStatus(controllerInstallation)
	}, equal)
}

// TryUpdateControllerInstallationStatus tries to update the status of the controllerInstallation matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerInstallation object. If the transformation
// yields a semantically equal ControllerInstallation, no update is done and the operation returns normally.
func TryUpdateControllerInstallationStatus(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error)) (*gardencorev1alpha1.ControllerInstallation, error) {
	return TryUpdateControllerInstallationStatusWithEqualFunc(g, backoff, meta, transform, func(cur, updated *gardencorev1alpha1.ControllerInstallation) bool {
		return equality.Semantic.DeepEqual(cur.Status, updated.Status)
	})
}

// CreateOrPatchControllerInstallation either creates the object or patches the existing one with the strategic merge patch type.
func CreateOrPatchControllerInstallation(g gardencore.Interface, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.ControllerInstallation) *gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error) {
	transformed := transform(&gardencorev1alpha1.ControllerInstallation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardencorev1alpha1.SchemeGroupVersion.String(),
			Kind:       "ControllerInstallation",
		},
		ObjectMeta: meta,
	})

	controllerInstallation, err := g.CoreV1alpha1().ControllerInstallations().Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return g.CoreV1alpha1().ControllerInstallations().Create(transformed)
		}
		return nil, err
	}
	return patchControllerInstallation(g, controllerInstallation, transform(controllerInstallation.DeepCopy()))
}

func patchControllerInstallation(g gardencore.Interface, oldObj, newObj *gardencorev1alpha1.ControllerInstallation) (*gardencorev1alpha1.ControllerInstallation, error) {
	patch, err := CreateTwoWayMergePatch(oldObj, newObj)
	if err != nil {
		return nil, err
	}

	if IsEmptyPatch(patch) {
		return oldObj, nil
	}

	return g.CoreV1alpha1().ControllerInstallations().Patch(oldObj.Name, types.StrategicMergePatchType, patch)
}
