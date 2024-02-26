// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package tokeninvalidator

import (
	"bytes"
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles admission requests and invalidates the static token in Secret resources related to ServiceAccounts.
type Handler struct {
	Logger logr.Logger
}

// Default invalidates the static token in Secret resources related to ServiceAccounts.
func (h *Handler) Default(ctx context.Context, obj runtime.Object) error {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("expected *corev1.Secret but got %T", obj)
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	log := h.Logger.WithValues("secret", kubernetesutils.ObjectKeyForCreateWebhooks(secret, req))

	if secret.Data == nil {
		log.Info("Secret's data is nil, nothing to be done")
		return nil
	}

	switch {
	case metav1.HasLabel(secret.ObjectMeta, resourcesv1alpha1.StaticTokenConsider):
		log.Info("Secret has 'consider' label, invalidating token")
		secret.Data[corev1.ServiceAccountTokenKey] = invalidToken

	case bytes.Equal(secret.Data[corev1.ServiceAccountTokenKey], invalidToken):
		log.Info("Secret has invalidated token and no 'consider' label, regenerating token")
		delete(secret.Data, corev1.ServiceAccountTokenKey)
	}

	return nil
}

var invalidToken = []byte("\u0000\u0000\u0000")
