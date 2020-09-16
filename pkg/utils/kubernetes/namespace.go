// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/mock/go/context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

func tryUpdateNamespace(
	ctx context.Context,
	k k8s.Interface,
	backoff wait.Backoff,
	meta metav1.ObjectMeta,
	transform func(*corev1.Namespace) (*corev1.Namespace, error),
	updateFunc func(k k8s.Interface, namespace *corev1.Namespace) (*corev1.Namespace, error),
	exitEarlyFunc func(cur, updated *corev1.Namespace) bool,
) (*corev1.Namespace, error) {
	var (
		result  *corev1.Namespace
		attempt int
	)

	err := retry.RetryOnConflict(backoff, func() (err error) {
		attempt++
		cur, err := k.CoreV1().Namespaces().Get(ctx, meta.Name, kubernetes.DefaultGetOptions())
		if err != nil {
			return err
		}

		updated, err := transform(cur.DeepCopy())
		if err != nil {
			return err
		}

		if exitEarlyFunc(cur, updated) {
			result = cur
			return nil
		}

		result, err = updateFunc(k, updated)
		if err != nil {
			logger.Logger.Errorf("Attempt %d failed to update Namespace %s due to %v", attempt, cur.Name, err)
		}
		return
	})
	if err != nil {
		logger.Logger.Errorf("Failed to update Namespace %s after %d attempts due to %v", meta.Name, attempt, err)
	}

	return result, err
}

// TryUpdateNamespace tries to update a namespace and retries the operation with the given <backoff>.
func TryUpdateNamespace(ctx context.Context, k k8s.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*corev1.Namespace) (*corev1.Namespace, error)) (*corev1.Namespace, error) {
	return tryUpdateNamespace(ctx, k, backoff, meta, transform, func(k k8s.Interface, namespace *corev1.Namespace) (*corev1.Namespace, error) {
		return k.CoreV1().Namespaces().Update(ctx, namespace, kubernetes.DefaultUpdateOptions())
	}, func(cur, updated *corev1.Namespace) bool {
		return equality.Semantic.DeepEqual(cur, updated)
	})
}

// TryUpdateNamespaceLabels tries to update a namespace's labels and retries the operation with the given <backoff>.
func TryUpdateNamespaceLabels(ctx context.Context, k k8s.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*corev1.Namespace) (*corev1.Namespace, error)) (*corev1.Namespace, error) {
	return tryUpdateNamespace(ctx, k, backoff, meta, transform, func(k k8s.Interface, namespace *corev1.Namespace) (*corev1.Namespace, error) {
		return k.CoreV1().Namespaces().Update(ctx, namespace, kubernetes.DefaultUpdateOptions())
	}, func(cur, updated *corev1.Namespace) bool {
		return equality.Semantic.DeepEqual(cur.Labels, updated.Labels)
	})
}
