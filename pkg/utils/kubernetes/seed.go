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

func tryUpdateSeed(
	ctx context.Context,
	g gardencore.Interface,
	backoff wait.Backoff,
	meta metav1.ObjectMeta,
	transform func(*gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error),
	updateFunc func(g gardencore.Interface, seed *gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error),
	equalFunc func(cur, updated *gardencorev1beta1.Seed) bool,
) (*gardencorev1beta1.Seed, error) {

	var (
		result  *gardencorev1beta1.Seed
		attempt int
	)
	err := retry.RetryOnConflict(backoff, func() (err error) {
		attempt++
		cur, err := g.CoreV1beta1().Seeds().Get(ctx, meta.Name, kubernetes.DefaultGetOptions())
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
			logger.Logger.Errorf("Attempt %d failed to update Seed %s due to %v", attempt, cur.Name, err)
		}
		return
	})
	if err != nil {
		logger.Logger.Errorf("Failed to update Seed %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return result, err
}

// TryUpdateSeedWithEqualFunc tries to update the status of the seed matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the Seed object. If the equal
// func concludes a semantically equal Seed, no update is done and the operation returns normally.
func TryUpdateSeedWithEqualFunc(ctx context.Context, g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error), equal func(cur, updated *gardencorev1beta1.Seed) bool) (*gardencorev1beta1.Seed, error) {
	return tryUpdateSeed(ctx, g, backoff, meta, transform, func(g gardencore.Interface, seed *gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
		return g.CoreV1beta1().Seeds().Update(ctx, seed, kubernetes.DefaultUpdateOptions())
	}, equal)
}

// TryUpdateSeed tries to update the status of the seed matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the Seed object. If the transformation
// yields a semantically equal Seed, no update is done and the operation returns normally.
func TryUpdateSeed(ctx context.Context, g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error)) (*gardencorev1beta1.Seed, error) {
	return TryUpdateSeedWithEqualFunc(ctx, g, backoff, meta, transform, func(cur, updated *gardencorev1beta1.Seed) bool {
		return equality.Semantic.DeepEqual(cur, updated)
	})
}

// TryUpdateSeedStatus tries to update the status of the seed matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the Seed object. If the transformation
// yields a semantically equal Seed (regarding Status), no update is done and the operation returns normally.
func TryUpdateSeedStatus(ctx context.Context, g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error)) (*gardencorev1beta1.Seed, error) {
	return tryUpdateSeed(ctx, g, backoff, meta, transform, func(g gardencore.Interface, seed *gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
		return g.CoreV1beta1().Seeds().UpdateStatus(ctx, seed, kubernetes.DefaultUpdateOptions())
	}, func(cur, updated *gardencorev1beta1.Seed) bool {
		return equality.Semantic.DeepEqual(cur.Status, updated.Status)
	})
}

// TryUpdateSeedConditions tries to update the status of the seed matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the Seed object. If the transformation
// yields a semantically equal Seed (regarding conditions), no update is done and the operation returns normally.
func TryUpdateSeedConditions(ctx context.Context, g gardencore.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error)) (*gardencorev1beta1.Seed, error) {
	return tryUpdateSeed(ctx, g, backoff, meta, transform, func(g gardencore.Interface, seed *gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
		return g.CoreV1beta1().Seeds().UpdateStatus(ctx, seed, kubernetes.DefaultUpdateOptions())
	}, func(cur, updated *gardencorev1beta1.Seed) bool {
		return equality.Semantic.DeepEqual(cur.Status.Conditions, updated.Status.Conditions)
	})
}
