// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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

func validate(obj runtime.Object) (admission.Warnings, error) {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil, fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", obj)
	}

	if errs := validation.ValidateGarden(garden); len(errs) > 0 {
		return nil, apierrors.NewInvalid(operatorv1alpha1.Kind("Garden"), garden.Name, errs)
	}

	return nil, nil
}

func validateUpdate(oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldGarden, ok := oldObj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil, fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", oldObj)
	}
	newGarden, ok := newObj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil, fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", newObj)
	}

	if errs := validation.ValidateGardenUpdate(oldGarden, newGarden); len(errs) > 0 {
		return nil, apierrors.NewInvalid(operatorv1alpha1.Kind("Garden"), newGarden.Name, errs)
	}

	return nil, nil
}

// ValidateCreate performs the validation.
func (h *Handler) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	otherGardensAlreadyExist, err := kubernetesutils.ResourcesExist(ctx, h.RuntimeClient, operatorv1alpha1.SchemeGroupVersion.WithKind("GardenList"))
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	if otherGardensAlreadyExist {
		return nil, apierrors.NewBadRequest("there can be only one operator.gardener.cloud/v1alpha1.Garden resource in the system at a time")
	}

	return validate(obj)
}

// ValidateUpdate performs the validation.
func (h *Handler) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return validateUpdate(oldObj, newObj)
}

// ValidateDelete performs the validation.
func (h *Handler) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return nil, fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", obj)
	}

	return nil, gardenerutils.CheckIfDeletionIsConfirmed(garden)
}
