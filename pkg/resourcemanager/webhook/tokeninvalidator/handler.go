// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"encoding/json"
	"net/http"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type tokenInvalidator struct {
	decoder *admission.Decoder
}

// NewHandler returns a new handler.
func NewHandler() admission.Handler {
	return &tokenInvalidator{}
}

func (w *tokenInvalidator) InjectDecoder(d *admission.Decoder) error {
	w.decoder = d
	return nil
}

func (w *tokenInvalidator) Handle(_ context.Context, req admission.Request) admission.Response {
	secret := &corev1.Secret{}
	if err := w.decoder.Decode(req, secret); err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	if secret.Data == nil {
		return admission.Allowed("data is nil")
	}

	switch {
	case metav1.HasLabel(secret.ObjectMeta, resourcesv1alpha1.StaticTokenConsider):
		secret.Data[corev1.ServiceAccountTokenKey] = invalidToken

	case bytes.Equal(secret.Data[corev1.ServiceAccountTokenKey], invalidToken):
		delete(secret.Data, corev1.ServiceAccountTokenKey)
	}

	marshaledSecret, err := json.Marshal(secret)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledSecret)
}

var invalidToken = []byte("\u0000\u0000\u0000")
