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
	"errors"
	"fmt"
	"net/http"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd"
)

type kubeconfigSecretValidator struct {
	codecs serializer.CodecFactory
	logger logrus.FieldLogger
}

const kubeconfigValidatorName = "kubeconfig_validator"

// NewValidateKubeconfigSecretsHandler creates a new handler for validating namespace deletions.
func NewValidateKubeconfigSecretsHandler() http.HandlerFunc {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))

	h := &kubeconfigSecretValidator{
		codecs: serializer.NewCodecFactory(scheme),
		logger: logger.NewFieldLogger(logger.Logger, "component", kubeconfigValidatorName),
	}
	return h.ValidateKubeconfigSecrets
}

// ValidateKubeconfigSecrets is a HTTP handler for validating whether a namespace deletion is allowed or not.
func (h *kubeconfigSecretValidator) ValidateKubeconfigSecrets(w http.ResponseWriter, r *http.Request) {
	var (
		deserializer   = h.codecs.UniversalDeserializer()
		receivedReview = &admissionv1beta1.AdmissionReview{}
		requestLogger  = logger.NewIDLogger(h.logger)
	)

	if err := DecodeAdmissionRequest(r, deserializer, receivedReview, maxRequestBody, requestLogger); err != nil {
		requestLogger.Errorf(err.Error())
		respond(w, errToAdmissionResponse(err))
		return
	}

	// If the request does not indicate the correct operations (CREATE, UPDATE) we allow the review without further doing.
	if receivedReview.Request.Operation != admissionv1beta1.Create && receivedReview.Request.Operation != admissionv1beta1.Update {
		respond(w, admissionResponse(true, ""))
		return
	}

	// Now that all checks have been passed we can actually validate the admission request.
	reviewResponse := h.admitSecrets(receivedReview.Request, deserializer)
	if !reviewResponse.Allowed && reviewResponse.Result != nil {
		requestLogger.Infof("Rejected '%s secret/%s/%s' request of user '%s': %v", receivedReview.Request.Operation, receivedReview.Request.Namespace, receivedReview.Request.Name, receivedReview.Request.UserInfo.Username, reviewResponse.Result.Message)
	}
	respond(w, reviewResponse)
}

// admitSecrets does only allow the request if the kubeconfig referenced in the secret does meet our standards.
func (h *kubeconfigSecretValidator) admitSecrets(request *admissionv1beta1.AdmissionRequest, decoder runtime.Decoder) *admissionv1beta1.AdmissionResponse {
	secretResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	if request.Resource != secretResource {
		return errToAdmissionResponse(fmt.Errorf("expect resource to be %s", secretResource))
	}

	if request.Object.Raw == nil {
		return errToAdmissionResponse(errors.New("expected secret object but got nothing"))
	}

	secret := &corev1.Secret{}
	if _, _, err := decoder.Decode(request.Object.Raw, nil, secret); err != nil {
		return errToAdmissionResponse(err)
	}

	kubeconfig, ok := secret.Data[kubernetes.KubeConfig]
	if !ok {
		return admissionResponse(true, "")
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return errToAdmissionResponse(err)
	}

	// Validate that the given kubeconfig doesn't have fields in its auth-info that are
	// not acceptable.
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return errToAdmissionResponse(err)
	}
	if err := kubernetes.ValidateConfig(rawConfig); err != nil {
		return errToAdmissionResponse(err)
	}

	return admissionResponse(true, "")
}
