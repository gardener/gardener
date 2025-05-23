// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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
func (h *Handler) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	var shoots []string

	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", newObj))
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if kubeConfig, ok := secret.Data[kubernetes.KubeConfig]; ok && len(kubeConfig) > 0 {
		h.Logger.Info("Secret has data `kubeconfig`, no need to check further", "name", secret.Name)
		return nil, nil
	}

	// lookup if secret is referenced by any shoot in the same namespace
	shootList := &gardencorev1beta1.ShootList{}
	if err := h.Client.List(ctx, shootList, client.InNamespace(req.Namespace)); err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("unable to list shoot in namespace: %v", req.Namespace))
	}

	for _, shoot := range shootList.Items {
		if shoot.Spec.Kubernetes.KubeAPIServer == nil {
			continue
		}

		if isReferencedInAdmissionPlugins(req.Name, shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins) ||
			isReferencedInStructuredAuthorization(req.Name, shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization) {
			shoots = append(shoots, shoot.Name)
		}
	}

	if len(shoots) > 0 {
		return nil, apierrors.NewForbidden(corev1.Resource("Secret"), req.Name, fmt.Errorf("data kubeconfig can't be removed from secret or set to empty because secret is in use by shoots: [%v]", strings.Join(shoots, ", ")))
	}

	return nil, nil
}

// ValidateDelete returns nil (not implemented by this handler).
func (h *Handler) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func isReferencedInAdmissionPlugins(secretName string, admissionPlugins []gardencorev1beta1.AdmissionPlugin) bool {
	for _, plugin := range admissionPlugins {
		if plugin.KubeconfigSecretName != nil && *plugin.KubeconfigSecretName == secretName {
			return true
		}
	}
	return false
}

func isReferencedInStructuredAuthorization(secretName string, structuredAuthorization *gardencorev1beta1.StructuredAuthorization) bool {
	if structuredAuthorization == nil {
		return false
	}

	for _, kubeconfig := range structuredAuthorization.Kubeconfigs {
		if kubeconfig.SecretName == secretName {
			return true
		}
	}

	return false
}
