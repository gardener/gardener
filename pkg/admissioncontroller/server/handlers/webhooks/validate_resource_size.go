// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"net/http"
	"strings"

	apisconfig "github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	confighelper "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/helper"
	"github.com/gardener/gardener/pkg/admissioncontroller/server/metrics"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
)

type objectSizeHandler struct {
	config *apisconfig.ResourceAdmissionConfiguration
	codecs serializer.CodecFactory
	logger logrus.FieldLogger
}

const resourceSizeValidatorName = "resource_size_validator"

// NewValidateResourceSizeHandler creates a new handler for validating the resource size of a request.
func NewValidateResourceSizeHandler(config *apisconfig.ResourceAdmissionConfiguration) http.HandlerFunc {
	scheme := runtime.NewScheme()
	utilruntime.Must(admissionregistrationv1beta1.AddToScheme(scheme))

	h := &objectSizeHandler{
		config: config,
		codecs: serializer.NewCodecFactory(scheme),
		logger: logger.NewFieldLogger(logger.Logger, "component", resourceSizeValidatorName),
	}
	return h.ValidateResourceSize
}

// ValidateResourceSize is a HTTP handler for validating whether the incoming resource does not exceed the configured limit.
func (h *objectSizeHandler) ValidateResourceSize(w http.ResponseWriter, r *http.Request) {
	var (
		deserializer   = h.codecs.UniversalDeserializer()
		receivedReview = &admissionv1beta1.AdmissionReview{}
		requestLogger  = logger.NewIDLogger(h.logger)
	)

	if err := DecodeAdmissionRequest(r, deserializer, receivedReview, maxRequestBody, requestLogger); err != nil {
		h.logger.Errorf(err.Error())
		respond(w, errToAdmissionResponse(err))

		metrics.InvalidWebhookRequest.WithLabelValues().Inc()
		return
	}

	logEntry := requestLogger.WithField("resource", receivedReview.Request.Resource).WithField("name", receivedReview.Request.Name)
	if receivedReview.Request.Namespace != "" {
		logEntry = logEntry.WithField("namespace", receivedReview.Request.Namespace)
	}

	// Now that all checks have been passed we can actually validate the admission request.
	reviewResponse := h.admit(receivedReview.Request, logEntry)
	if !reviewResponse.Allowed && reviewResponse.Result != nil {
		logEntry.Infof("Rejected request of user '%s': %v", receivedReview.Request.UserInfo.Username, reviewResponse.Result.Message)

		metrics.RejectedResources.WithLabelValues(
			fmt.Sprint(receivedReview.Request.Operation),
			receivedReview.Request.Kind.Kind,
			receivedReview.Request.Namespace,
			metrics.ReasonSizeExceeded,
		).Inc()
	}
	respond(w, reviewResponse)
}

// admit does only allow the request if the object in the request does not exceed a configured limit.
func (h *objectSizeHandler) admit(request *admissionv1beta1.AdmissionRequest, logEntry logrus.FieldLogger) *admissionv1beta1.AdmissionResponse {
	if request.SubResource != "" {
		return admissionResponse(true, "")
	}

	if isUnrestrictedUser(request.UserInfo, h.config.UnrestrictedSubjects) {
		return admissionResponse(true, "")
	}

	requested := &request.Resource
	if request.RequestResource != nil {
		// Use original requested resource if available, see doc string of `admissionv1beta1.RequestResource`.
		requested = request.RequestResource
	}

	limit := findLimitForGVR(h.config.Limits, requested)
	if limit == nil {
		return admissionResponse(true, "")
	}

	objectSize := len(request.Object.Raw)
	if limit.CmpInt64(int64(objectSize)) == -1 {
		msg := fmt.Sprintf("Maximum resource size exceeded! Size in request: %d bytes. Max allowed: %s", objectSize, limit)
		if h.config.OperationMode == nil || *h.config.OperationMode == apisconfig.AdmissionModeBlock {
			return admissionResponse(false, msg)
		}
		logEntry.Infof("Request will be denied in blocking mode: %s", msg)
	}

	return admissionResponse(true, "")
}

func serviceAccountMatch(userInfo authenticationv1.UserInfo, subjects []rbacv1.Subject) bool {
	for _, subject := range subjects {
		if subject.Kind == rbacv1.ServiceAccountKind {
			if confighelper.ServiceAccountMatches(subject, userInfo) {
				return true
			}
		}
	}
	return false
}

func userMatch(userInfo authenticationv1.UserInfo, subjects []rbacv1.Subject) bool {
	for _, subject := range subjects {
		var match bool
		switch subject.Kind {
		case rbacv1.UserKind:
			match = confighelper.UserMatches(subject, userInfo)
		case rbacv1.GroupKind:
			match = confighelper.UserGroupMatches(subject, userInfo)
		}
		if match {
			return true
		}
	}
	return false
}

func isUnrestrictedUser(userInfo authenticationv1.UserInfo, subjects []rbacv1.Subject) bool {
	isServiceAccount := strings.HasPrefix(userInfo.Username, serviceaccount.ServiceAccountUsernamePrefix)
	if isServiceAccount {
		return serviceAccountMatch(userInfo, subjects)
	}
	return userMatch(userInfo, subjects)
}

func findLimitForGVR(limits []apisconfig.ResourceLimit, gvr *metav1.GroupVersionResource) *resource.Quantity {
	for _, limit := range limits {
		size := limit.Size
		if confighelper.APIGroupMatches(limit, gvr.Group) &&
			confighelper.VersionMatches(limit, gvr.Version) &&
			confighelper.ResourceMatches(limit, gvr.Resource) {
			return &size
		}
	}
	return nil
}
