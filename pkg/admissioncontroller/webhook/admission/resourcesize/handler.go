// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcesize

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	admissioncontrollerhelper "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/admissioncontroller/metrics"
)

const (
	// metricReasonSizeExceeded is a metric reason value for a reason when an object size was exceeded.
	metricReasonSizeExceeded = "Size Exceeded"

	// metricReasonCountExceeded is a metric reason value for a reason when resource count was exceeded.
	metricReasonCountExceeded = "Count Exceeded"
)

// Handler checks the resource sizes.
type Handler struct {
	Logger     logr.Logger
	Config     *admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration
	APIReader  client.Reader
	RESTMapper meta.RESTMapper
}

// Handle checks the resource sizes.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	var err error

	switch req.Operation {
	case admissionv1.Create:
		err = h.handle(ctx, req)
	case admissionv1.Update:
		err = h.handle(ctx, req)
	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unknown operation request %q", req.Operation))
	}

	if err != nil {
		var apiStatus apierrors.APIStatus
		if errors.As(err, &apiStatus) {
			status := apiStatus.Status()
			return admission.Response{AdmissionResponse: admissionv1.AdmissionResponse{Allowed: false, Result: &status}}
		}
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

func (h *Handler) handle(ctx context.Context, req admission.Request) error {
	log := h.Logger.WithValues("user", req.UserInfo.Username, "resource", req.Resource, "name", req.Name)
	if req.Namespace != "" {
		log = log.WithValues("namespace", req.Namespace)
	}

	if isUnrestrictedUser(req.UserInfo, h.Config.UnrestrictedSubjects) {
		return nil
	}

	requestedResource := &req.Resource
	if req.RequestResource != nil {
		// Use original requested requestedResource if available, see doc string of `admissionv1.RequestResource`.
		requestedResource = req.RequestResource
	}

	limit, count := findRestrictionsForGVR(h.Config.Limits, requestedResource)
	if limit == nil && count == nil {
		return nil
	}

	// Handle resource count limits
	if count != nil {
		if err := h.handleCountLimit(ctx, req, log, requestedResource, count); err != nil {
			return fmt.Errorf("error handling count limit: %w", err)
		}
	}

	// Handle resource size limits
	if limit != nil {
		if err := h.handleSizeLimit(req, log, limit); err != nil {
			return fmt.Errorf("error handling size limit: %w", err)
		}
	}

	return nil
}

func (h *Handler) handleSizeLimit(req admission.Request, log logr.Logger, limit *resource.Quantity) error {
	objectSize, err := relevantObjectSize(req.Object.Raw)
	if err != nil {
		return err
	}
	if limit.CmpInt64(objectSize) == -1 {
		if h.Config.OperationMode == nil || *h.Config.OperationMode == admissioncontrollerconfigv1alpha1.AdmissionModeBlock {
			log.Info("Maximum resource size exceeded, rejected request", "requestObjectSize", objectSize, "limit", limit)
			metrics.RejectedResources.WithLabelValues(
				fmt.Sprint(req.Operation),
				req.Kind.Kind,
				req.Namespace,
				metricReasonSizeExceeded,
			).Inc()
			return apierrors.NewForbidden(schema.GroupResource{Group: req.Resource.Group, Resource: req.Resource.Resource}, req.Name, fmt.Errorf("maximum resource size exceeded! Size in request: %d bytes, max allowed: %s", objectSize, limit))
		}

		log.Info("Maximum resource size exceeded, request would be denied in blocking mode", "requestObjectSize", objectSize, "limit", limit)
	}

	return nil
}

func (h *Handler) handleCountLimit(ctx context.Context, req admission.Request, log logr.Logger, requestedResource *metav1.GroupVersionResource, count *int64) error {
	if req.Namespace != "" {
		// We only want to restrict non-namespaced resources
		// namespaced resources can be restricted by ResourceQuotas
		log.Info("Skipping count check for namespaced resource", "resource", requestedResource.String())
		return nil
	}

	// Only count for CREATE operations, not UPDATE
	if req.Operation != admissionv1.Create {
		return nil
	}

	exceedsLimit, err := h.existingResourcesExceedLimit(ctx, req, requestedResource, ptr.Deref(count, 0))
	if err != nil {
		return fmt.Errorf("failed to determine if count exceeds limit: %w", err)
	}

	if !exceedsLimit {
		return nil
	}

	if h.Config.OperationMode == nil || *h.Config.OperationMode == admissioncontrollerconfigv1alpha1.AdmissionModeBlock {
		log.Info("Maximum resource count exceeded, rejected request", "limit", *count)
		metrics.RejectedResources.WithLabelValues(
			fmt.Sprint(req.Operation),
			req.Kind.Kind,
			req.Namespace,
			metricReasonCountExceeded,
		).Inc()
		return apierrors.NewForbidden(schema.GroupResource{Group: req.Resource.Group, Resource: req.Resource.Resource}, req.Name, fmt.Errorf("maximum resource count exceeded! max allowed: %d", ptr.Deref(count, 0)))
	}

	log.Info("Maximum resource count exceeded, request would be denied in blocking mode", "limit", ptr.Deref(count, 0))
	return nil
}

func relevantObjectSize(rawObject []byte) (int64, error) {
	var obj map[string]any
	err := json.Unmarshal(rawObject, &obj)
	if err != nil {
		return 0, err
	}
	delete(obj, "status")
	if obj["metadata"] != nil {
		delete(obj["metadata"].(map[string]any), "managedFields")
	}
	marshalled, err := json.Marshal(obj)
	return int64(len(marshalled)), err
}

func serviceAccountMatch(userInfo authenticationv1.UserInfo, subjects []rbacv1.Subject) bool {
	for _, subject := range subjects {
		if subject.Kind == rbacv1.ServiceAccountKind {
			if admissioncontrollerhelper.ServiceAccountMatches(subject, userInfo) {
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
			match = admissioncontrollerhelper.UserMatches(subject, userInfo)
		case rbacv1.GroupKind:
			match = admissioncontrollerhelper.UserGroupMatches(subject, userInfo)
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

func findRestrictionsForGVR(limits []admissioncontrollerconfigv1alpha1.ResourceLimit, gvr *metav1.GroupVersionResource) (*resource.Quantity, *int64) {
	for _, limit := range limits {
		if admissioncontrollerhelper.APIGroupMatches(limit, gvr.Group) &&
			admissioncontrollerhelper.VersionMatches(limit, gvr.Version) &&
			admissioncontrollerhelper.ResourceMatches(limit, gvr.Resource) {
			return limit.Size, limit.Count
		}
	}
	return nil, nil
}

func (h *Handler) getKindFromGVR(gvr *metav1.GroupVersionResource) (string, error) {
	gvk, err := h.RESTMapper.KindFor(schema.GroupVersionResource{
		Group:    gvr.Group,
		Version:  gvr.Version,
		Resource: gvr.Resource,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get Kind for GVR %s: %w", gvr.String(), err)
	}
	return gvk.Kind, nil
}

func (h *Handler) existingResourcesExceedLimit(ctx context.Context, _ admission.Request, gvr *metav1.GroupVersionResource, limit int64) (bool, error) {
	timeoutContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get the proper Kind for this resource
	kind, err := h.getKindFromGVR(gvr)
	if err != nil {
		return false, fmt.Errorf("failed to list resources for calculating object count: %w", err)
	}

	list := &metav1.PartialObjectMetadataList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    kind + "List",
	})
	// List all resources of this type cluster-wide (since we only count non-namespaced resources)
	if err := h.APIReader.List(timeoutContext, list, client.Limit(limit+1)); err != nil {
		return false, fmt.Errorf("failed to list resources for counting: %v", err)
	}

	return int64(len(list.Items)) >= limit, nil
}
