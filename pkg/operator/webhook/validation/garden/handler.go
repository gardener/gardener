// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/validation"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler performs validation.
type Handler struct {
	Logger        logr.Logger
	RuntimeClient client.Client
}

// forbiddenFinalizersOnCreation is a list of finalizers which are forbidden to be specified on Garden creation.
var forbiddenFinalizersOnCreation = sets.New(
	operatorv1alpha1.FinalizerName,
	v1beta1constants.ReferenceProtectionFinalizerName,
)

// ValidateCreate performs the validation.
func (h *Handler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	otherGardensAlreadyExist, err := kubernetesutils.ResourcesExist(ctx, h.RuntimeClient, &operatorv1alpha1.GardenList{}, h.RuntimeClient.Scheme())
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	if otherGardensAlreadyExist {
		return nil, apierrors.NewBadRequest("there can be only one operator.gardener.cloud/v1alpha1.Garden resource in the system at a time")
	}

	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil, fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", obj)
	}

	for _, finalizer := range garden.Finalizers {
		if forbiddenFinalizersOnCreation.Has(finalizer) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("finalizer %q cannot be added on creation", finalizer))
		}
	}

	extensionList := &operatorv1alpha1.ExtensionList{}
	if err := h.RuntimeClient.List(ctx, extensionList); err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if errs := validation.ValidateGarden(garden, extensionList.Items); len(errs) > 0 {
		return nil, apierrors.NewInvalid(operatorv1alpha1.Kind("Garden"), garden.Name, errs)
	}

	return nil, nil
}

// ValidateUpdate performs the validation.
func (h *Handler) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldGarden, ok := oldObj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil, fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", oldObj)
	}
	newGarden, ok := newObj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil, fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", newObj)
	}

	extensionList := &operatorv1alpha1.ExtensionList{}
	if err := h.RuntimeClient.List(ctx, extensionList); err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if errs := validation.ValidateGardenUpdate(oldGarden, newGarden, extensionList.Items); len(errs) > 0 {
		return nil, apierrors.NewInvalid(operatorv1alpha1.Kind("Garden"), newGarden.Name, errs)
	}

	return nil, nil
}

// ValidateDelete performs the validation.
func (h *Handler) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil, fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", obj)
	}

	return nil, gardenerutils.CheckIfDeletionIsConfirmed(garden)
}
