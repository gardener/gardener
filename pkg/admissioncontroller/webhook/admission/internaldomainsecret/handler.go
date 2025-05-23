// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internaldomainsecret

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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
	Scheme    *runtime.Scheme
}

// ValidateCreate performs the check.
func (h *Handler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", obj))
	}

	seedName := gardenerutils.ComputeSeedName(secret.Namespace)
	if secret.Namespace != v1beta1constants.GardenNamespace && seedName == "" {
		return nil, nil
	}

	exists, err := h.internalDomainSecretExists(ctx, secret.Namespace)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	if exists {
		return nil, apierrors.NewConflict(schema.GroupResource{Group: corev1.GroupName, Resource: "Secret"}, secret.Name, errors.New("cannot create internal domain secret because there can be only one secret with the 'internal-domain' secret role per namespace"))
	}

	if _, _, _, err := gardenerutils.GetDomainInfoFromAnnotations(secret.Annotations); err != nil {
		return nil, apierrors.NewBadRequest(err.Error())
	}

	return nil, nil
}

// ValidateUpdate performs the check.
func (h *Handler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", newObj))
	}

	oldSecret, ok := oldObj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", oldObj))
	}

	seedName := gardenerutils.ComputeSeedName(secret.Namespace)
	if secret.Namespace != v1beta1constants.GardenNamespace && seedName == "" {
		return nil, nil
	}

	// If secret was newly labeled with gardener.cloud/role=internal-domain then check whether another internal domain
	// secret already exists.
	if oldSecret.Labels[v1beta1constants.GardenRole] != v1beta1constants.GardenRoleInternalDomain &&
		secret.Labels[v1beta1constants.GardenRole] == v1beta1constants.GardenRoleInternalDomain {
		exists, err := h.internalDomainSecretExists(ctx, secret.Namespace)
		if err != nil {
			return nil, apierrors.NewInternalError(err)
		}
		if exists {
			return nil, apierrors.NewConflict(schema.GroupResource{Group: corev1.GroupName, Resource: "Secret"}, secret.Name, errors.New("cannot update secret because there can be only one secret with the 'internal-domain' secret role per namespace"))
		}
	}

	_, oldDomain, _, err := gardenerutils.GetDomainInfoFromAnnotations(oldSecret.Annotations)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	_, newDomain, _, err := gardenerutils.GetDomainInfoFromAnnotations(secret.Annotations)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if oldDomain != newDomain {
		atLeastOneShoot, err := h.atLeastOneShootExists(ctx, seedName)
		if err != nil {
			return nil, apierrors.NewInternalError(err)
		}
		if atLeastOneShoot {
			return nil, apierrors.NewForbidden(schema.GroupResource{Group: corev1.GroupName, Resource: "Secret"}, secret.Name, errors.New("cannot change domain because there are still shoots left in the system"))
		}
	}

	return nil, nil
}

// ValidateDelete performs the check.
func (h *Handler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", obj))
	}

	seedName := gardenerutils.ComputeSeedName(secret.Namespace)
	if secret.Namespace != v1beta1constants.GardenNamespace && seedName == "" {
		return nil, nil
	}

	atLeastOneShoot, err := h.atLeastOneShootExists(ctx, seedName)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	if atLeastOneShoot {
		return nil, apierrors.NewForbidden(schema.GroupResource{Group: corev1.GroupName, Resource: "Secret"}, secret.Name, errors.New("cannot delete internal domain secret because there are still shoots left in the system"))
	}

	return nil, nil
}

func (h *Handler) atLeastOneShootExists(ctx context.Context, seedName string) (bool, error) {
	var listOpts []client.ListOption
	if seedName != "" {
		listOpts = append(listOpts, client.MatchingFields{
			gardencore.ShootSeedName: seedName,
		})
	}

	return kubernetesutils.ResourcesExist(ctx, h.APIReader, &gardencorev1beta1.ShootList{}, h.Scheme, listOpts...)
}

func (h *Handler) internalDomainSecretExists(ctx context.Context, namespace string) (bool, error) {
	return kubernetesutils.ResourcesExist(ctx, h.APIReader, &corev1.SecretList{}, h.Scheme, client.InNamespace(namespace), client.MatchingLabels{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain,
	})
}
