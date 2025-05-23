// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedeletion

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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles namespace deletions.
type Handler struct {
	Logger    logr.Logger
	APIReader client.Reader
	Client    client.Client
}

// ValidateCreate returns nil (not implemented by this handler).
func (h *Handler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate returns nil (not implemented by this handler).
func (h *Handler) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete validates the namespace deletion.
func (h *Handler) ValidateDelete(ctx context.Context, _ runtime.Object) (admission.Warnings, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	if err := h.admitNamespace(ctx, req.Name); err != nil {
		h.Logger.Info("Rejected namespace deletion", "user", req.UserInfo.Username, "reason", err.Error())
		return nil, err
	}

	return nil, nil
}

// admitNamespace does only allow the request if no Shoots exist in this specific namespace anymore.
func (h *Handler) admitNamespace(ctx context.Context, namespaceName string) error {
	// Determine project for given namespace.
	// TODO: we should use a direct lookup here, as we might falsely allow the request, if our cache is
	// out of sync and doesn't know about the project. We should use a field selector for looking up the project
	// belonging to a given namespace.
	project, namespace, err := gardenerutils.ProjectAndNamespaceFromReader(ctx, h.Client, namespaceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return apierrors.NewInternalError(err)
	}

	if project == nil {
		return nil
	}

	switch {
	case namespace.DeletionTimestamp != nil:
		return nil

	case project.DeletionTimestamp != nil:
		// if project is marked for deletion we need to wait until all shoots in the namespace are gone
		namespaceInUse, err := kubernetesutils.ResourcesExist(ctx, h.APIReader, &gardencorev1beta1.ShootList{}, h.Client.Scheme(), client.InNamespace(namespace.Name))
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		if !namespaceInUse {
			return nil
		}

		return apierrors.NewForbidden(schema.GroupResource{Group: corev1.GroupName, Resource: "Namespace"}, namespace.Name, fmt.Errorf("deletion of namespace %q is not permitted (it still contains Shoots)", namespace.Name))
	}

	// Namespace is not yet marked for deletion and project is not marked as well. We do not admit and respond that
	// namespace deletion is only allowed via project deletion.
	return apierrors.NewForbidden(schema.GroupResource{Group: corev1.GroupName, Resource: "Namespace"}, namespace.Name, fmt.Errorf("direct deletion of namespace %q is not permitted (you must delete the corresponding project %q)", namespace.Name, project.Name))
}
