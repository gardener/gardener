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

package resourcesize

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	apisconfig "github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	confighelper "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/helper"
	"github.com/gardener/gardener/pkg/admissioncontroller/metrics"
	acadmission "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "resource_size_validator"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/validate-resource-size"

	// metricReasonSizeExceeded is a metric reason value for a reason when an object size was exceeded.
	metricReasonSizeExceeded = "Size Exceeded"
)

// New creates a new webhook handler validating that the resource size of a request doesn't exceed the configured
// limits.
func New(logger logr.Logger, config *apisconfig.ResourceAdmissionConfiguration) *handler {
	return &handler{logger: logger, config: config}
}

type handler struct {
	logger  logr.Logger
	config  *apisconfig.ResourceAdmissionConfiguration
	decoder *admission.Decoder
}

var _ admission.Handler = &handler{}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(_ context.Context, request admission.Request) admission.Response {
	requestID, err := utils.GenerateRandomString(8)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	requestLogger := h.logger.WithValues(
		logger.IDFieldName, requestID,
		"user", request.UserInfo.Username,
		"resource", request.Resource, "name", request.Name,
	)
	if request.Namespace != "" {
		requestLogger = requestLogger.WithValues("namespace", request.Namespace)
	}

	response := h.admitRequestSize(request, requestLogger)
	if !response.Allowed {
		metrics.RejectedResources.WithLabelValues(
			fmt.Sprint(request.Operation),
			request.Kind.Kind,
			request.Namespace,
			metricReasonSizeExceeded,
		).Inc()
	}

	return response
}

func (h *handler) admitRequestSize(request admission.Request, requestLogger logr.Logger) admission.Response {
	if request.SubResource != "" {
		return acadmission.Allowed("subresources are not handled")
	}

	if isUnrestrictedUser(request.UserInfo, h.config.UnrestrictedSubjects) {
		return acadmission.Allowed("user is unrestricted")
	}

	requestedResource := &request.Resource
	if request.RequestResource != nil {
		// Use original requested requestedResource if available, see doc string of `admissionv1.RequestResource`.
		requestedResource = request.RequestResource
	}

	limit := findLimitForGVR(h.config.Limits, requestedResource)
	if limit == nil {
		return acadmission.Allowed("no limit configured for requested resource")
	}

	objectSize := len(request.Object.Raw)
	if limit.CmpInt64(int64(objectSize)) == -1 {
		if h.config.OperationMode == nil || *h.config.OperationMode == apisconfig.AdmissionModeBlock {
			requestLogger.Info("maximum resource size exceeded, rejected request",
				"requestObjectSize", objectSize, "limit", limit)

			return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("Maximum resource size exceeded! Size in request: %d bytes. Max allowed: %s", objectSize, limit))
		}
		requestLogger.Info("maximum resource size exceeded, request will be denied in blocking mode",
			"requestObjectSize", objectSize, "limit", limit)
	}

	return acadmission.Allowed("resource size ok")
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
