// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shootkubeconfigsecretref

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// Handler validates shoot kubeconfig secrets.
type Handler struct {
	Logger logr.Logger
	Client client.Reader
}

// ValidateCreate returns nil (not implemented by this handler).
func (h *Handler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate validates that the kubeconfig is not removed from kubeconfig secrets referenced in Shoot resources.
func (h *Handler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newSecret, ok := newObj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", newObj))
	}

	oldSecret, ok := oldObj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", oldObj))
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	// If the new secret still has a non-empty kubeconfig, nothing is removed and there is no need to check further.
	if kubeConfig, ok := newSecret.Data[kubernetes.KubeConfig]; ok && len(kubeConfig) > 0 {
		h.Logger.Info("Secret has data `kubeconfig`, no need to check further", "name", newSecret.Name)
		return nil, nil
	}

	// If the old secret did not have a non-empty kubeconfig either, nothing is being removed and there is no need to
	// proceed further.
	if oldKubeConfig, ok := oldSecret.Data[kubernetes.KubeConfig]; !ok || len(oldKubeConfig) == 0 {
		return nil, nil
	}

	// Check if the secret is referenced by any shoot in the same namespace via field-selector-backed indexes.
	shoots, err := h.referencingShootNames(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if shoots.Len() > 0 {
		return nil, apierrors.NewForbidden(corev1.Resource("Secret"), req.Name, fmt.Errorf("data kubeconfig can't be removed from secret or set to empty because secret is in use by shoots: [%v]", strings.Join(sets.List(shoots), ", ")))
	}

	return nil, nil
}

// ValidateDelete returns nil (not implemented by this handler).
func (h *Handler) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// referencingShootNames returns the names of the shoots in the given namespace that reference the secret with the
// given name either via an admission plugin kubeconfig or via a structured authorization kubeconfig.
func (h *Handler) referencingShootNames(ctx context.Context, namespace, secretName string) (sets.Set[string], error) {
	shoots := sets.New[string]()

	for _, field := range []string{
		gardencore.ShootAdmissionPluginKubeconfigSecretName,
		gardencore.ShootStructuredAuthorizationKubeconfigSecretName,
	} {
		shootList := &gardencorev1beta1.ShootList{}
		if err := h.Client.List(ctx, shootList, client.InNamespace(namespace), client.MatchingFields{field: secretName}); err != nil {
			return nil, fmt.Errorf("unable to list shoots in namespace %q: %w", namespace, err)
		}

		for _, shoot := range shootList.Items {
			shoots.Insert(shoot.Name)
		}
	}

	return shoots, nil
}
