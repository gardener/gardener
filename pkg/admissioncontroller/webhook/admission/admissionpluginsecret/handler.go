// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package admissionpluginsecret

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

// Handler validates admission plugins secret.
type Handler struct {
	Logger logr.Logger
	Client client.Reader
}

// ValidateCreate returns nil (not implemented by this handler).
func (h *Handler) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateUpdate validate an admission plugins secret.
func (h *Handler) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) error {
	var shoots []string

	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected *corev1.Secret but got %T", newObj))
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	if kubeConfig, ok := secret.Data[kubernetes.KubeConfig]; ok && len(kubeConfig) > 0 {
		h.Logger.Info("Secret has data `kubeconfig` no need to check further", "name", secret.Name)
		return nil
	}

	// lookup if secret is referenced by any shoot in the same namespace
	shootList := &gardencorev1beta1.ShootList{}
	if err := h.Client.List(ctx, shootList, client.InNamespace(req.Namespace)); err != nil {
		return apierrors.NewInternalError(fmt.Errorf("unable to list shoot in namespace: %v", req.Namespace))
	}

	for _, shoot := range shootList.Items {
		if shoot.Spec.Kubernetes.KubeAPIServer != nil {
			for _, plugin := range shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins {
				if plugin.KubeconfigSecretName != nil && *plugin.KubeconfigSecretName == req.Name {
					shoots = append(shoots, shoot.Name)
				}
			}
		}
	}

	if len(shoots) > 0 {
		return apierrors.NewForbidden(corev1.Resource("Secret"), req.Name, fmt.Errorf("data kubeconfig can't be removed from secret or set to empty because secret is in use by shoots: [%v]", strings.Join(shoots, ", ")))
	}

	return nil
}

// ValidateDelete returns nil (not implemented by this handler).
func (h *Handler) ValidateDelete(_ context.Context, _ runtime.Object) error {
	return nil
}
