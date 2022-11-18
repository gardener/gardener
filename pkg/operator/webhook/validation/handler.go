// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/validation"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
)

// Handler checks, if the secrets contains a kubeconfig and denies kubeconfigs with invalid fields (e.g. tokenFile or
// exec).
type Handler struct {
	Logger logr.Logger
}

// ValidateCreate performs the validation.
func (h *Handler) ValidateCreate(_ context.Context, obj runtime.Object) error {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", obj)
	}

	if errs := validation.ValidateGarden(garden); len(errs) > 0 {
		return apierrors.NewInvalid(operatorv1alpha1.Kind("Garden"), garden.Name, errs)
	}

	return nil
}

// ValidateUpdate performs the validation.
func (h *Handler) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) error {
	return h.ValidateCreate(ctx, newObj)
}

// ValidateDelete performs the validation.
func (h *Handler) ValidateDelete(_ context.Context, obj runtime.Object) error {
	garden, ok := obj.(*operatorv1alpha1.Garden)
	if !ok {
		return fmt.Errorf("expected *operatorv1alpha1.Garden but got %T", obj)
	}

	return gutil.CheckIfDeletionIsConfirmed(garden)
}
