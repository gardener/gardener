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

func tryUpdatePlant(
	g gardencore.Interface,
	backoff wait.Backoff,
	meta metav1.ObjectMeta,
	transform func(*gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error),
	updateFunc func(g gardencore.Interface, plant *gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error),
	equalFunc func(cur, updated *gardencorev1alpha1.Plant) bool,
) (*gardencorev1alpha1.Plant, error) {

	var (
		result  *gardencorev1alpha1.Plant
		attempt int
	)
	err := retry.RetryOnConflict(backoff, func() (err error) {
		attempt++
		cur, err := g.CoreV1alpha1().Plants(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
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
			logger.Logger.Errorf("Attempt %d failed to update Plant %s due to %v", attempt, cur.Name, err)
		}
		return
	})
	if err != nil {
		logger.Logger.Errorf("Failed to updated Plants %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return result, err
}

// TryUpdatePlantConditions tries to update the status of the Plant matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the Plant object. If the transformation
// yields a semantically equal Plant (regarding conditions), no update is done and the operation returns normally.
func TryUpdatePlantConditions(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(plant *gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error)) (*gardencorev1alpha1.Plant, error) {
	return tryUpdatePlant(g, backoff, meta, transform, func(g gardencore.Interface, plant *gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error) {
		return g.CoreV1alpha1().Plants(plant.Namespace).UpdateStatus(plant)
	}, func(cur, updated *gardencorev1alpha1.Plant) bool {
		return equality.Semantic.DeepEqual(cur.Status.Conditions, updated.Status.Conditions)
	})
}

// TryUpdatePlantStatusWithEqualFunc tries to update the status of the Plant matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the Plant object. If the equal
// func concludes a semantically equal Plant, no update is done and the operation returns normally.
func TryUpdatePlantStatusWithEqualFunc(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error), equal func(cur, updated *gardencorev1alpha1.Plant) bool) (*gardencorev1alpha1.Plant, error) {
	return tryUpdatePlant(g, backoff, meta, transform, func(g gardencore.Interface, plant *gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error) {
		return g.CoreV1alpha1().Plants(plant.Namespace).UpdateStatus(plant)
	}, equal)
}

// TryUpdatePlantWithEqualFunc tries to update the status of the Plant matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the Plant object. If the equal
// func concludes a semantically equal Plant, no update is done and the operation returns normally.
func TryUpdatePlantWithEqualFunc(g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error), equal func(cur, updated *gardencorev1alpha1.Plant) bool) (*gardencorev1alpha1.Plant, error) {
	return tryUpdatePlant(g, backoff, meta, transform, func(g gardencore.Interface, plant *gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error) {
		return g.CoreV1alpha1().Plants(plant.Namespace).Update(plant)
	}, equal)
}

// CreateOrPatchPlant either creates the object or patches the existing one with the strategic merge patch type.
func CreateOrPatchPlant(g gardencore.Interface, meta metav1.ObjectMeta, transform func(*gardencorev1alpha1.Plant) *gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error) {
	transformed := transform(&gardencorev1alpha1.Plant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardencorev1alpha1.SchemeGroupVersion.String(),
			Kind:       "Plant",
		},
		ObjectMeta: meta,
	})

	plant, err := g.CoreV1alpha1().Plants(metav1.NamespaceAll).Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return g.CoreV1alpha1().Plants(metav1.NamespaceAll).Create(transformed)
		}
		return nil, err
	}
	return patchPlant(g, plant, transform(plant.DeepCopy()))
}

func patchPlant(g gardencore.Interface, oldObj, newObj *gardencorev1alpha1.Plant) (*gardencorev1alpha1.Plant, error) {
	patch, err := CreateTwoWayMergePatch(oldObj, newObj)
	if err != nil {
		return nil, err
	}

	if IsEmptyPatch(patch) {
		return oldObj, nil
	}

	return g.CoreV1alpha1().Plants(metav1.NamespaceAll).Patch(oldObj.Name, types.StrategicMergePatchType, patch)
}
