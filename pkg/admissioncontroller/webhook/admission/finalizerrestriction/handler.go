// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizerrestriction

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	clientauthorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	seedidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/seed"
	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/shoot/storage"
)

// Handler protects finalizers on system resources from being removed by users other than gardener system users,
// system service accounts, or system administrators.
type Handler struct {
	Logger                logr.Logger
	Decoder               admission.Decoder
	ProtectedFinalizers   sets.Set[string]
	SubjectAccessReviewer clientauthorizationv1.SubjectAccessReviewInterface
}

// Handle allows the request unless it removes a protected finalizer. Removal of a protected finalizer is only
// permitted for gardener system users, dedicated system service accounts, and system administrators.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := h.Logger.WithValues(
		"requestUID", req.UID,
		"operation", req.Operation,
		"kind", req.Kind.Kind,
		"resource", req.Resource.Resource,
		"user", req.UserInfo.Username,
		"groups", req.UserInfo.Groups,
	)

	// Checks various user types in the 'gardener.cloud:system:*' and 'system:serviceaccounts' groups
	_, _, _, userType := shootidentity.FromAuthenticationV1UserInfo(req.UserInfo)
	if userType == gardenletidentity.UserTypeGardenlet {
		log.V(1).Info("Allowing request", "reason", "gardenlet is allowed to modify protected finalizers")
		return admission.Allowed("gardenlet is allowed to modify protected finalizer(s)")
	}
	if userType == gardenletidentity.UserTypeExtension {
		log.V(1).Info("Allowing request", "reason", "extension is allowed to modify protected finalizers")
		return admission.Allowed("extension is allowed to modify protected finalizer(s)")
	}
	if userType == gardenletidentity.UserTypeGardenadm {
		log.V(1).Info("Allowing request", "reason", "gardenadm is allowed to modify protected finalizers")
		return admission.Allowed("gardenadm is allowed to modify protected finalizer(s)")
	}
	_, _, seedUserType := seedidentity.FromAuthenticationV1UserInfo(req.UserInfo)
	if seedUserType == gardenletidentity.UserTypeGardenlet {
		log.V(1).Info("Allowing request", "reason", "gardenlet is allowed to modify protected finalizers")
		return admission.Allowed("gardenlet is allowed to modify protected finalizer(s)")
	}
	if seedUserType == gardenletidentity.UserTypeExtension {
		log.V(1).Info("Allowing request", "reason", "extension is allowed to modify protected finalizers")
		return admission.Allowed("extension is allowed to modify protected finalizer(s)")
	}

	// Allow all requests from the 'system:serviceaccounts:kube-system' group
	groups := sets.New(req.UserInfo.Groups...)
	if groups.Has(serviceaccount.MakeNamespaceGroupName(metav1.NamespaceSystem)) {
		log.V(1).Info("Allowing request", "reason", "kube-system service account is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("kube-system service account %q is allowed to modify protected finalizer(s)", req.UserInfo.Username))
	}
	// Also allow 'system:masters' group
	if groups.Has(user.SystemPrivilegedGroup) {
		log.V(1).Info("Allowing request", "reason", "cluster administrator is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("cluster administrator user %q is allowed to modify protected finalizer(s)", req.UserInfo.Username))
	}

	// None of the obvious user types / groups matched, need to check if finalizers were actually modified and then check admin user group
	oldObj := &metav1.PartialObjectMetadata{}
	if len(req.OldObject.Raw) > 0 {
		if err := json.Unmarshal(req.OldObject.Raw, oldObj); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}
	oldFinalizers := sets.New(oldObj.GetFinalizers()...)
	newObj := &metav1.PartialObjectMetadata{}
	if len(req.Object.Raw) > 0 {
		if err := json.Unmarshal(req.Object.Raw, newObj); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}
	newFinalizers := sets.New(newObj.GetFinalizers()...)
	changedFinalizers := h.ProtectedFinalizers.Intersection(oldFinalizers.SymmetricDifference(newFinalizers))
	if changedFinalizers.Len() == 0 {
		return admission.Allowed("No protected finalizers were modified")
	}

	obj := newObj
	if len(req.Object.Raw) == 0 {
		obj = oldObj
	}
	log = log.WithValues(
		"namespace", obj.GetNamespace(),
		"name", obj.GetName(),
		"addedFinalizers", changedFinalizers.Intersection(newFinalizers).UnsortedList(),
		"removedFinalizers", changedFinalizers.Intersection(oldFinalizers).UnsortedList(),
	)

	extra := make(map[string][]string, len(req.UserInfo.Extra))
	for k, v := range req.UserInfo.Extra {
		extra[k] = v
	}
	adminGroups, err := storage.GetAdminUserGroups(ctx, &user.DefaultInfo{
		Name:   req.UserInfo.Username,
		UID:    req.UserInfo.UID,
		Groups: req.UserInfo.Groups,
		Extra:  extra,
	}, h.SubjectAccessReviewer)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if slices.Contains(adminGroups, v1beta1constants.ShootSystemAdminsGroupName) {
		log.V(1).Info("Allowing request", "reason", "system administrator is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("system administrator user %q is allowed to modify protected finalizer(s) %v", req.UserInfo.Username, changedFinalizers.UnsortedList()))
	}

	log.Info("Denying request", "reason", "user is not allowed to modify the protected finalizers")
	return admission.Denied(fmt.Sprintf("user %q is not allowed to modify protected finalizer(s) %v", req.UserInfo.Username, changedFinalizers.UnsortedList()))
}
