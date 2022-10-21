// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package namespacedeletion

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles namespace deletions.
type Handler struct {
	Logger    logr.Logger
	APIReader client.Reader
	Client    client.Reader
}

// ValidateCreate returns nil (not implemented by this handler).
func (h *Handler) ValidateCreate(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateUpdate returns nil (not implemented by this handler).
func (h *Handler) ValidateUpdate(_ context.Context, _, _ runtime.Object) error {
	return nil
}

// ValidateDelete validates the namespace deletion.
func (h *Handler) ValidateDelete(ctx context.Context, _ runtime.Object) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	if err := h.admitNamespace(ctx, req.Name); err != nil {
		h.Logger.Info("Rejected namespace deletion", "user", req.UserInfo.Username, "reason", err.Error())
		return err
	}

	return nil
}

// admitNamespace does only allow the request if no Shoots exist in this specific namespace anymore.
func (h *Handler) admitNamespace(ctx context.Context, namespaceName string) error {
	// Determine project for given namespace.
	// TODO: we should use a direct lookup here, as we might falsely allow the request, if our cache is
	// out of sync and doesn't know about the project. We should use a field selector for looking up the project
	// belonging to a given namespace.
	project, namespace, err := gutil.ProjectAndNamespaceFromReader(ctx, h.Client, namespaceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if project == nil {
		return nil
	}

	switch {
	case namespace.DeletionTimestamp != nil:
		return nil

	case project.DeletionTimestamp != nil:
		// if project is marked for deletion we need to wait until all shoots in the namespace are gone
		namespaceInUse, err := kutil.IsNamespaceInUse(ctx, h.APIReader, namespace.Name, gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
		if err != nil {
			return err
		}

		if !namespaceInUse {
			return nil
		}

		return fmt.Errorf("deletion of namespace %q is not permitted (it still contains Shoots)", namespace.Name)
	}

	// Namespace is not yet marked for deletion and project is not marked as well. We do not admit and respond that
	// namespace deletion is only allowed via project deletion.
	return fmt.Errorf("direct deletion of namespace %q is not permitted (you must delete the corresponding project %q)", namespace.Name, project.Name)
}
