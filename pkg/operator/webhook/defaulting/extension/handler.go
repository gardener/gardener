//  SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	extensionutils "github.com/gardener/gardener/pkg/operator/controller/extension/utils"
)

// Handler performs defaulting.
type Handler struct {
	Logger logr.Logger
}

// Default performs the defaulting.
func (h *Handler) Default(_ context.Context, obj runtime.Object) error {
	extension, ok := obj.(*operatorv1alpha1.Extension)
	if !ok {
		return fmt.Errorf("expected *operatorv1alpha1.Extension but got %T", obj)
	}

	// merge extensions with defaults
	err := extensionutils.ApplyExtensionSpec(extension)
	if err != nil {
		return fmt.Errorf("error merging extension spec: %w", err)
	}

	return nil
}
