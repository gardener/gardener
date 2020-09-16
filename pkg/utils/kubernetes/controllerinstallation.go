// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencore "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

func tryUpdateControllerInstallation(
	ctx context.Context,
	g gardencore.Interface,
	backoff wait.Backoff,
	meta metav1.ObjectMeta,
	transform func(*gardencorev1beta1.ControllerInstallation) (*gardencorev1beta1.ControllerInstallation, error),
	updateFunc func(g gardencore.Interface, controllerInstallation *gardencorev1beta1.ControllerInstallation) (*gardencorev1beta1.ControllerInstallation, error),
	equalFunc func(cur, updated *gardencorev1beta1.ControllerInstallation) bool,
) (*gardencorev1beta1.ControllerInstallation, error) {

	var (
		result  *gardencorev1beta1.ControllerInstallation
		attempt int
	)
	err := retry.RetryOnConflict(backoff, func() (err error) {
		attempt++
		cur, err := g.CoreV1beta1().ControllerInstallations().Get(ctx, meta.Name, kubernetes.DefaultGetOptions())
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
		logger.Logger.Errorf("Failed to update ControllerInstallation %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return result, err
}

// TryUpdateControllerInstallationWithEqualFunc tries to update the status of the controllerInstallation matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerInstallation object. If the equal
// func concludes a semantically equal ControllerInstallation, no update is done and the operation returns normally.
func TryUpdateControllerInstallationWithEqualFunc(ctx context.Context, g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.ControllerInstallation) (*gardencorev1beta1.ControllerInstallation, error), equal func(cur, updated *gardencorev1beta1.ControllerInstallation) bool) (*gardencorev1beta1.ControllerInstallation, error) {
	return tryUpdateControllerInstallation(ctx, g, backoff, meta, transform, func(g gardencore.Interface, controllerInstallation *gardencorev1beta1.ControllerInstallation) (*gardencorev1beta1.ControllerInstallation, error) {
		return g.CoreV1beta1().ControllerInstallations().Update(ctx, controllerInstallation, kubernetes.DefaultUpdateOptions())
	}, equal)
}

// TryUpdateControllerInstallation tries to update the status of the controllerInstallation matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerInstallation object. If the transformation
// yields a semantically equal ControllerInstallation, no update is done and the operation returns normally.
func TryUpdateControllerInstallation(ctx context.Context, g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.ControllerInstallation) (*gardencorev1beta1.ControllerInstallation, error)) (*gardencorev1beta1.ControllerInstallation, error) {
	return TryUpdateControllerInstallationWithEqualFunc(ctx, g, backoff, meta, transform, func(cur, updated *gardencorev1beta1.ControllerInstallation) bool {
		return equality.Semantic.DeepEqual(cur, updated)
	})
}

// TryUpdateControllerInstallationStatusWithEqualFunc tries to update the status of the controllerInstallation matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerInstallation object. If the equal
// func concludes a semantically equal ControllerInstallation, no update is done and the operation returns normally.
func TryUpdateControllerInstallationStatusWithEqualFunc(ctx context.Context, g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.ControllerInstallation) (*gardencorev1beta1.ControllerInstallation, error), equal func(cur, updated *gardencorev1beta1.ControllerInstallation) bool) (*gardencorev1beta1.ControllerInstallation, error) {
	return tryUpdateControllerInstallation(ctx, g, backoff, meta, transform, func(g gardencore.Interface, controllerInstallation *gardencorev1beta1.ControllerInstallation) (*gardencorev1beta1.ControllerInstallation, error) {
		return g.CoreV1beta1().ControllerInstallations().UpdateStatus(ctx, controllerInstallation, kubernetes.DefaultUpdateOptions())
	}, equal)
}

// TryUpdateControllerInstallationStatus tries to update the status of the controllerInstallation matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the ControllerInstallation object. If the transformation
// yields a semantically equal ControllerInstallation, no update is done and the operation returns normally.
func TryUpdateControllerInstallationStatus(ctx context.Context, g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.ControllerInstallation) (*gardencorev1beta1.ControllerInstallation, error)) (*gardencorev1beta1.ControllerInstallation, error) {
	return TryUpdateControllerInstallationStatusWithEqualFunc(ctx, g, backoff, meta, transform, func(cur, updated *gardencorev1beta1.ControllerInstallation) bool {
		return equality.Semantic.DeepEqual(cur.Status, updated.Status)
	})
}
