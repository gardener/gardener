// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package internaldomainsecret

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler validates the immutability of the internal domain secret.
type Handler struct {
	Logger    logr.Logger
	APIReader client.Reader
}

// ValidateCreate performs the check.
func (h *Handler) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", obj))
	}

	seedName := gardenerutils.ComputeSeedName(secret.Namespace)
	if secret.Namespace != v1beta1constants.GardenNamespace && seedName == "" {
		return nil
	}

	exists, err := h.internalDomainSecretExists(ctx, secret.Namespace)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	if exists {
		return apierrors.NewConflict(schema.GroupResource{Group: corev1.GroupName, Resource: "Secret"}, secret.Name, fmt.Errorf("cannot create internal domain secret because there can be only one secret with the 'internal-domain' secret role per namespace"))
	}

	if _, _, _, err := gardenerutils.GetDomainInfoFromAnnotations(secret.Annotations); err != nil {
		return apierrors.NewBadRequest(err.Error())
	}

	return nil
}

// ValidateUpdate performs the check.
func (h *Handler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", newObj))
	}

	oldSecret, ok := oldObj.(*corev1.Secret)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", oldObj))
	}

	seedName := gardenerutils.ComputeSeedName(secret.Namespace)
	if secret.Namespace != v1beta1constants.GardenNamespace && seedName == "" {
		return nil
	}

	// If secret was newly labeled with gardener.cloud/role=internal-domain then check whether another internal domain
	// secret already exists.
	if oldSecret.Labels[v1beta1constants.GardenRole] != v1beta1constants.GardenRoleInternalDomain &&
		secret.Labels[v1beta1constants.GardenRole] == v1beta1constants.GardenRoleInternalDomain {
		exists, err := h.internalDomainSecretExists(ctx, secret.Namespace)
		if err != nil {
			return apierrors.NewInternalError(err)
		}
		if exists {
			return apierrors.NewConflict(schema.GroupResource{Group: corev1.GroupName, Resource: "Secret"}, secret.Name, fmt.Errorf("cannot update secret because there can be only one secret with the 'internal-domain' secret role per namespace"))
		}
	}

	_, oldDomain, _, err := gardenerutils.GetDomainInfoFromAnnotations(oldSecret.Annotations)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	_, newDomain, _, err := gardenerutils.GetDomainInfoFromAnnotations(secret.Annotations)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	if oldDomain != newDomain {
		atLeastOneShoot, err := h.atLeastOneShootExists(ctx, seedName)
		if err != nil {
			return apierrors.NewInternalError(err)
		}
		if atLeastOneShoot {
			return apierrors.NewForbidden(schema.GroupResource{Group: corev1.GroupName, Resource: "Secret"}, secret.Name, fmt.Errorf("cannot change domain because there are still shoots left in the system"))
		}
	}

	return nil
}

// ValidateDelete performs the check.
func (h *Handler) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", obj))
	}

	seedName := gardenerutils.ComputeSeedName(secret.Namespace)
	if secret.Namespace != v1beta1constants.GardenNamespace && seedName == "" {
		return nil
	}

	atLeastOneShoot, err := h.atLeastOneShootExists(ctx, seedName)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	if atLeastOneShoot {
		return apierrors.NewForbidden(schema.GroupResource{Group: corev1.GroupName, Resource: "Secret"}, secret.Name, fmt.Errorf("cannot delete internal domain secret because there are still shoots left in the system"))
	}

	return nil
}

func (h *Handler) atLeastOneShootExists(ctx context.Context, seedName string) (bool, error) {
	var listOpts []client.ListOption
	if seedName != "" {
		listOpts = append(listOpts, client.MatchingFields{
			gardencore.ShootSeedName: seedName,
		})
	}

	return kubernetesutils.ResourcesExist(ctx, h.APIReader, gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"), listOpts...)
}

func (h *Handler) internalDomainSecretExists(ctx context.Context, namespace string) (bool, error) {
	return kubernetesutils.ResourcesExist(ctx, h.APIReader, corev1.SchemeGroupVersion.WithKind("SecretList"), client.InNamespace(namespace), client.MatchingLabels{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain,
	})
}
