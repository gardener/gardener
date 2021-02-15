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

package seed

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) seedEnqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	c.seedQueue.Add(key)
}

func (c *Controller) seedAdd(obj interface{}) {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return
	}

	c.seedEnqueue(seed)
}

type reconciler struct {
	clientMap    clientmap.ClientMap
	secretLister corev1listers.SecretLister
	seedLister   gardencorelisters.SeedLister
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	seedObj, err := r.seedLister.Get(req.Name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Infof("[SEED] Stopping operations for Seed %s since it has been deleted", req.Name)
		return reconcileResult(nil)
	}
	if err != nil {
		logger.Logger.Infof("[SEED] %s - unable to retrieve object from store: %v", req.Name, err)
		return reconcileResult(err)
	}

	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcileResult(fmt.Errorf("failed to get garden client: %w", err))
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("seed-%s", seedObj.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(seedObj, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed")),
			},
		},
	}

	// TODO: use controllerutil.CreateOrUpdate once cache is graduated to beta https://github.com/gardener/gardener/issues/2822
	if err := gardenClient.Client().Create(ctx, namespace); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return reconcileResult(err)
		}
		if _, err2 := kutil.TryUpdateNamespace(ctx, gardenClient.Kubernetes(), retry.DefaultRetry, metav1.ObjectMeta{Name: namespace.Name}, func(ns *corev1.Namespace) (*corev1.Namespace, error) {
			ns.OwnerReferences = namespace.OwnerReferences
			return ns, nil
		}); err2 != nil {
			return reconcileResult(err2)
		}
	}

	syncedSecrets, err := r.syncGardenSecrets(ctx, gardenClient, namespace)
	if err != nil {
		return reconcileResult(fmt.Errorf("failed to sync garden secrets: %v", err))
	}

	if err := r.cleanupStaleSecrets(ctx, gardenClient, syncedSecrets, namespace.Name); err != nil {
		return reconcileResult(fmt.Errorf("failed to clean up secrets in seed namespace: %v", err))
	}

	return reconcileResult(nil)
}

func (r *reconciler) cleanupStaleSecrets(ctx context.Context, gardenClient kubernetes.Interface, existingSecrets []string, namespace string) error {
	var fns []flow.TaskFn
	exclude := sets.NewString(existingSecrets...)

	secrets, err := r.secretLister.Secrets(namespace).List(gardenRoleSelector)
	if err != nil {
		return err
	}

	for _, s := range secrets {
		secret := s
		if exclude.Has(secret.Name) {
			continue
		}
		fns = append(fns, func(ctx context.Context) error {
			if err := gardenClient.Client().Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
				return err
			}
			return nil
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (r *reconciler) syncGardenSecrets(ctx context.Context, gardenClient kubernetes.Interface, namespace *corev1.Namespace) ([]string, error) {
	secretsGardenRole, err := r.secretLister.Secrets(v1beta1constants.GardenNamespace).List(gardenRoleSelector)
	if err != nil {
		return nil, err
	}

	var (
		fns         []flow.TaskFn
		secretNames []string
	)
	for _, s := range secretsGardenRole {
		secret := s
		secretNames = append(secretNames, secret.Name)
		fns = append(fns, func(ctx context.Context) error {
			seedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: secret.Annotations,
					Labels:      secret.Labels,
					Name:        secret.Name,
					Namespace:   namespace.Name,
				},
				Data: secret.Data,
			}
			// TODO: use controllerutil.CreateOrUpdate once cache is graduated to beta https://github.com/gardener/gardener/issues/2822
			if err := gardenClient.Client().Create(ctx, seedSecret); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return err
				}
				_, err2 := kutil.TryUpdateSecret(ctx, gardenClient.Kubernetes(), retry.DefaultRetry, seedSecret.ObjectMeta, func(obj *corev1.Secret) (*corev1.Secret, error) {
					obj.Annotations = seedSecret.Annotations
					obj.Labels = seedSecret.Labels
					obj.Data = seedSecret.Data
					return obj, nil
				})
				return err2
			}
			return nil
		})
	}

	for _, f := range fns {
		if err := f(ctx); err != nil {
			return nil, err
		}
	}

	//return secretNames, flow.Parallel(fns...)(ctx)
	return secretNames, nil
}

// NewDefaultControl returns a new instance of the default implementation that
// implements the documented semantics for seeds.
// You should use an instance returned from NewDefaultControl() for any scenario other than testing.
func NewDefaultControl(
	clientMap clientmap.ClientMap,
	secretLister corev1listers.SecretLister,
	seedLister gardencorelisters.SeedLister,
) *reconciler {
	return &reconciler{
		clientMap:    clientMap,
		secretLister: secretLister,
		seedLister:   seedLister,
	}
}
