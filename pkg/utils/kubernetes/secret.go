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

package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

func tryUpdateSecret(
	ctx context.Context,
	k k8s.Interface,
	backoff wait.Backoff,
	meta metav1.ObjectMeta,
	transform func(*corev1.Secret) (*corev1.Secret, error),
	updateFunc func(k k8s.Interface, secret *corev1.Secret) (*corev1.Secret, error),
	equalFunc func(cur, updated *corev1.Secret) bool,
) (*corev1.Secret, error) {

	var (
		result  *corev1.Secret
		attempt int
	)
	err := retry.RetryOnConflict(backoff, func() (err error) {
		attempt++
		cur, err := k.CoreV1().Secrets(meta.Namespace).Get(ctx, meta.Name, kubernetes.DefaultGetOptions())
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

		result, err = updateFunc(k, updated)
		if err != nil {
			logger.Logger.Errorf("Attempt %d failed to update Secret %s/%s due to %v", attempt, cur.Namespace, cur.Name, err)
		}
		return
	})
	if err != nil {
		logger.Logger.Errorf("Failed to update Secret %s/%s after %d attempts due to %v", meta.Namespace, meta.Name, attempt, err)
	}
	return result, err
}

// TryUpdateSecret tries to update the secret matching the given <meta>.
// It retries with the given <backoff> characteristics as long as it gets Conflict errors.
// The transformation function is applied to the current state of the Secret object. If the transformation
// yields a semantically equal Secret, no update is done and the operation returns normally.
func TryUpdateSecret(ctx context.Context, k k8s.Interface, backoff wait.Backoff, meta metav1.ObjectMeta, transform func(*corev1.Secret) (*corev1.Secret, error)) (*corev1.Secret, error) {
	return tryUpdateSecret(ctx, k, backoff, meta, transform, func(k k8s.Interface, secret *corev1.Secret) (*corev1.Secret, error) {
		return k.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, kubernetes.DefaultUpdateOptions())
	}, func(cur, updated *corev1.Secret) bool {
		return equality.Semantic.DeepEqual(cur, updated)
	})
}
