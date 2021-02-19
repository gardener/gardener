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

package webhooks

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/metrics"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	kubeconfigValidatorName = "kubeconfig_validator"

	// reasonInvalidKubeconfig is a StatusReason that will be sent as part of the webhook's response if the secret
	// contains an invalid kubeconfig
	reasonInvalidKubeconfig = metav1.StatusReason("InvalidKubeconfig")
)

var secretGVK = metav1.GroupVersionKind{Group: "", Kind: "Secret", Version: "v1"}

var _ admission.Handler = &KubeconfigSecretValidator{}

// KubeconfigSecretValidator is a webhook handler validating CREATE and UPDATE requests on secrets. It checks, if the
// secrets contains a kubeconfig and denies kubeconfigs with invalid fields (e.g. tokenFile or exec).
type KubeconfigSecretValidator struct {
	logger  logr.Logger
	decoder *admission.Decoder
}

// InjectLogger injects a logger into the handler.
func (h *KubeconfigSecretValidator) InjectLogger(l logr.Logger) error {
	h.logger = l.WithName(kubeconfigValidatorName)
	return nil
}

// InjectDecoder injects a decoder capable of decoding objects included in admission requests.
func (h *KubeconfigSecretValidator) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// Handle implements the webhook handler for kubeconfig secret validation.
func (h *KubeconfigSecretValidator) Handle(_ context.Context, request admission.Request) admission.Response {
	// If the request does not indicate the correct operations (CREATE, UPDATE) we allow the review without further doing.
	if request.Operation != admissionv1.Create && request.Operation != admissionv1.Update {
		return admissionAllowed("operation is neither CREATE nor UPDATE")
	}
	if request.Kind != secretGVK {
		return admissionAllowed("resource is not corev1.Secret")
	}
	if request.SubResource != "" {
		return admissionAllowed("subresources on secrets are not handled")
	}

	requestID, err := utils.GenerateRandomString(8)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	requestLogger := h.logger.WithValues(logger.IDFieldName, requestID)

	secret := &corev1.Secret{}
	if err := h.decoder.Decode(request, secret); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	response := h.admitKubeconfigSecret(secret)
	if !response.Allowed && response.Result != nil {
		response.Result.Reason = reasonInvalidKubeconfig

		requestLogger.Info("rejected secret", "reason", response.Result.Reason,
			"message", response.Result.Message, "operation", request.Operation,
			"namespace", request.Namespace, "name", request.Name, "username", request.UserInfo.Username)

		metrics.RejectedResources.WithLabelValues(
			string(request.Operation),
			request.Kind.Kind,
			request.Namespace,
			metrics.ReasonRejectedKubeconfig,
		).Inc()
	}
	return response
}

// admitKubeconfigSecret checks if the given secret contains a kubeconfig, which meets our security requirements.
func (h *KubeconfigSecretValidator) admitKubeconfigSecret(secret *corev1.Secret) admission.Response {
	kubeconfig, ok := secret.Data[kubernetes.KubeConfig]
	if !ok {
		return admissionAllowed("secret does not contain kubeconfig")
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	// Validate that the given kubeconfig doesn't have fields in its auth-info that are
	// not acceptable.
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	if err := kubernetes.ValidateConfig(rawConfig); err != nil {
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("secret contains invalid kubeconfig: %w", err))
	}

	return admissionAllowed("kubeconfig is valid")
}
