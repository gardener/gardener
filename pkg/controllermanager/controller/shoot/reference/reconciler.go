// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package reference

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler checks the shoot in the given request for references to further objects in order to protect them from
// deletions as long as they are still referenced.
type Reconciler struct {
	Client client.Client
	Config config.ShootReferenceControllerConfiguration
}

// Reconcile checks the shoot in the given request for references to further objects in order to protect them from
// deletions as long as they are still referenced.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// Iterate over all user secrets in project namespace and check if they can be released.
	if err := r.releaseUnreferencedSecrets(ctx, log, shoot); err != nil {
		return reconcile.Result{}, err
	}

	// Iterate over all user configmaps in project namespace and check if they can be released.
	if err := r.releaseUnreferencedConfigMaps(ctx, log, shoot); err != nil {
		return reconcile.Result{}, err
	}

	// Remove finalizer from shoot in case it's being deleted and not handled by Gardener anymore.
	if shoot.DeletionTimestamp != nil && !controllerutil.ContainsFinalizer(shoot, gardencorev1beta1.GardenerName) {
		if controllerutil.ContainsFinalizer(shoot, v1beta1constants.ReferenceProtectionFinalizerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.Client, shoot, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
		return reconcile.Result{}, nil
	}

	// Add finalizer to referenced secrets that are not managed by Gardener.
	addedFinalizerToSecret, err := r.handleReferencedSecrets(ctx, log, r.Client, shoot)
	if err != nil {
		return reconcile.Result{}, err
	}

	addedFinalizerToConfigMap, err := r.handleReferencedConfigMap(ctx, log, r.Client, shoot)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Manage finalizers on shoot.
	var (
		hasFinalizer   = controllerutil.ContainsFinalizer(shoot, v1beta1constants.ReferenceProtectionFinalizerName)
		needsFinalizer = addedFinalizerToSecret || addedFinalizerToConfigMap
	)

	if needsFinalizer && !hasFinalizer {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, shoot, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
		return reconcile.Result{}, nil
	}

	if !needsFinalizer && hasFinalizer {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.Client, shoot, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) handleReferencedSecrets(ctx context.Context, log logr.Logger, c client.Client, shoot *gardencorev1beta1.Shoot) (bool, error) {
	var (
		fns         []flow.TaskFn
		added       = uint32(0)
		secretNames = append(
			secretNamesForDNSProviders(shoot),
			namesForReferencedResources(shoot, "Secret")...,
		)
	)

	for _, secretName := range secretNames {
		name := secretName
		fns = append(fns, func(ctx context.Context) error {
			secret := &corev1.Secret{}
			if err := c.Get(ctx, kubernetesutils.Key(shoot.Namespace, name), secret); err != nil {
				return err
			}

			// Don't handle Gardener managed secrets.
			if _, ok := secret.Labels[v1beta1constants.GardenRole]; ok {
				return nil
			}

			atomic.StoreUint32(&added, 1)

			if !controllerutil.ContainsFinalizer(secret, v1beta1constants.ReferenceProtectionFinalizerName) {
				log.Info("Adding finalizer to secret", "secret", client.ObjectKeyFromObject(secret))
				if err := controllerutils.AddFinalizers(ctx, r.Client, secret, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
					return fmt.Errorf("failed to add finalizer to secret: %w", err)
				}
			}

			return nil
		})
	}

	return added != 0, flow.Parallel(fns...)(ctx)
}

func (r *Reconciler) handleReferencedConfigMap(ctx context.Context, log logr.Logger, c client.Client, shoot *gardencorev1beta1.Shoot) (bool, error) {
	var (
		fns            []flow.TaskFn
		added          = uint32(0)
		configMapNames = namesForReferencedResources(shoot, "ConfigMap")
	)

	if configMapRef := getAuditPolicyConfigMapRef(shoot.Spec.Kubernetes.KubeAPIServer); configMapRef != nil {
		configMapNames = append(configMapNames, configMapRef.Name)
	}

	for _, configMapName := range configMapNames {
		name := configMapName
		fns = append(fns, func(ctx context.Context) error {
			configMap := &corev1.ConfigMap{}
			if err := c.Get(ctx, kubernetesutils.Key(shoot.Namespace, name), configMap); err != nil {
				return err
			}

			atomic.StoreUint32(&added, 1)

			if !controllerutil.ContainsFinalizer(configMap, v1beta1constants.ReferenceProtectionFinalizerName) {
				log.Info("Adding finalizer to ConfigMap", "configMap", client.ObjectKeyFromObject(configMap))
				if err := controllerutils.AddFinalizers(ctx, r.Client, configMap, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
					return fmt.Errorf("failed to add finalizer to ConfigMap: %w", err)
				}
			}

			return nil
		})
	}

	return added != 0, flow.Parallel(fns...)(ctx)
}

func (r *Reconciler) releaseUnreferencedSecrets(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	secrets, err := r.getUnreferencedSecrets(ctx, shoot)
	if err != nil {
		return err
	}

	var fns []flow.TaskFn
	for _, secret := range secrets {
		s := secret
		fns = append(fns, func(ctx context.Context) error {
			if controllerutil.ContainsFinalizer(&s, v1beta1constants.ReferenceProtectionFinalizerName) {
				log.Info("Removing finalizer from secret", "secret", client.ObjectKeyFromObject(&s))
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, &s, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
					return fmt.Errorf("failed to remove finalizer from secret: %w", err)
				}
			}
			return nil
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (r *Reconciler) releaseUnreferencedConfigMaps(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	configMaps, err := r.getUnreferencedConfigMaps(ctx, shoot)
	if err != nil {
		return err
	}

	var fns []flow.TaskFn
	for _, configMap := range configMaps {
		cm := configMap
		fns = append(fns, func(ctx context.Context) error {
			if controllerutil.ContainsFinalizer(&cm, v1beta1constants.ReferenceProtectionFinalizerName) {
				log.Info("Removing finalizer from ConfigMap", "configMap", client.ObjectKeyFromObject(&cm))
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, &cm, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
					return fmt.Errorf("failed to remove finalizer from ConfigMap: %w", err)
				}
			}
			return nil
		})

	}
	return flow.Parallel(fns...)(ctx)
}

var (
	noGardenRole = utils.MustNewRequirement(v1beta1constants.GardenRole, selection.DoesNotExist)

	// UserManagedSelector is a selector for objects which are managed by users and not created by Gardener.
	UserManagedSelector = client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(noGardenRole)}
)

func (r *Reconciler) getUnreferencedSecrets(ctx context.Context, shoot *gardencorev1beta1.Shoot) ([]corev1.Secret, error) {
	namespace := shoot.Namespace

	secrets := &corev1.SecretList{}
	if err := r.Client.List(ctx, secrets, client.InNamespace(namespace), UserManagedSelector); err != nil {
		return nil, err
	}

	shoots := &gardencorev1beta1.ShootList{}
	if err := r.Client.List(ctx, shoots, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	referencedSecrets := sets.New[string]()
	for _, s := range shoots.Items {
		// Ignore own references if shoot is in deletion and references are not needed any more by Gardener.
		if s.Name == shoot.Name && shoot.DeletionTimestamp != nil && !controllerutil.ContainsFinalizer(&s, gardencorev1beta1.GardenerName) {
			continue
		}
		referencedSecrets.Insert(secretNamesForDNSProviders(&s)...)
		referencedSecrets.Insert(namesForReferencedResources(&s, "Secret")...)
	}

	var secretsToRelease []corev1.Secret
	for _, secret := range secrets.Items {
		if !controllerutil.ContainsFinalizer(&secret, v1beta1constants.ReferenceProtectionFinalizerName) {
			continue
		}
		if referencedSecrets.Has(secret.Name) {
			continue
		}
		secretsToRelease = append(secretsToRelease, secret)
	}

	return secretsToRelease, nil
}

func (r *Reconciler) getUnreferencedConfigMaps(ctx context.Context, shoot *gardencorev1beta1.Shoot) ([]corev1.ConfigMap, error) {
	namespace := shoot.Namespace

	configMaps := &corev1.ConfigMapList{}
	if err := r.Client.List(ctx, configMaps, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	// Exit early if there are no ConfigMaps at all in the namespace
	if len(configMaps.Items) == 0 {
		return nil, nil
	}

	shoots := &gardencorev1beta1.ShootList{}
	if err := r.Client.List(ctx, shoots, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	referencedConfigMaps := sets.New[string]()
	for _, s := range shoots.Items {
		// Ignore own references if shoot is in deletion and references are not needed any more by Gardener.
		if s.Name == shoot.Name && shoot.DeletionTimestamp != nil && !controllerutil.ContainsFinalizer(&s, gardencorev1beta1.GardenerName) {
			continue
		}

		if configMapRef := getAuditPolicyConfigMapRef(s.Spec.Kubernetes.KubeAPIServer); configMapRef != nil {
			referencedConfigMaps.Insert(configMapRef.Name)
		}
		referencedConfigMaps.Insert(namesForReferencedResources(&s, "ConfigMap")...)
	}

	var configMapsToRelease []corev1.ConfigMap
	for _, configMap := range configMaps.Items {
		if !controllerutil.ContainsFinalizer(&configMap, v1beta1constants.ReferenceProtectionFinalizerName) {
			continue
		}
		if referencedConfigMaps.Has(configMap.Name) {
			continue
		}
		configMapsToRelease = append(configMapsToRelease, configMap)
	}

	return configMapsToRelease, nil
}

func secretNamesForDNSProviders(shoot *gardencorev1beta1.Shoot) []string {
	if shoot.Spec.DNS == nil {
		return nil
	}
	var names = make([]string, 0, len(shoot.Spec.DNS.Providers))
	for _, provider := range shoot.Spec.DNS.Providers {
		if provider.SecretName == nil {
			continue
		}
		names = append(names, *provider.SecretName)
	}

	return names
}

func namesForReferencedResources(shoot *gardencorev1beta1.Shoot, kind string) []string {
	var names []string
	for _, ref := range shoot.Spec.Resources {
		if ref.ResourceRef.APIVersion == "v1" && ref.ResourceRef.Kind == kind {
			names = append(names, ref.ResourceRef.Name)
		}
	}
	return names
}

func getAuditPolicyConfigMapRef(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) *corev1.ObjectReference {
	if apiServerConfig != nil &&
		apiServerConfig.AuditConfig != nil &&
		apiServerConfig.AuditConfig.AuditPolicy != nil &&
		apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil {

		return apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef
	}

	return nil
}
