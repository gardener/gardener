// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespace

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler performs validation.
type Handler struct {
	Logger        logr.Logger
	RuntimeClient client.Client
}

// ValidateCreate performs the validation.
func (h *Handler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate performs the validation.
func (h *Handler) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete performs the validation.
func (h *Handler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil, fmt.Errorf("expected *corev1.Namespace but got %T", obj)
	}

	gardenExists, err := kubernetesutils.ResourcesExist(ctx, h.RuntimeClient, &operatorv1alpha1.GardenList{}, h.RuntimeClient.Scheme())
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	if gardenExists {
		h.Logger.Info("Preventing deletion attempt of namespace because a Garden resource exists", "namespace", namespace.Name)
		return nil, apierrors.NewForbidden(corev1.Resource("namespaces"), namespace.Name, fmt.Errorf("deletion of namespace %q is forbidden while a Garden resource exists - delete it first before deleting the namespace", namespace.Name))
	}

	return nil, nil
}
