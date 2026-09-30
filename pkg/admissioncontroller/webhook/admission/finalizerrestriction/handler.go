// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizerrestriction

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	clientauthorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	seedidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/seed"
	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
)

// Handler protects finalizers on system resources from being removed by users other than the gardenlet,
// system service accounts, or system administrators.
type Handler struct {
	Logger                logr.Logger
	Decoder               admission.Decoder
	ProtectedFinalizers   sets.Set[string]
	SubjectAccessReviewer clientauthorizationv1.SubjectAccessReviewInterface
}

// Handle allows the request unless it removes a protected finalizer. Removal of a protected finalizer is only
// permitted for the gardenlet, dedicated system service accounts, and system administrators.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	// TODO: Discuss: we could do the user checks first, which would probably be more performant, since we dont need to unmarshal the object for most valid requests.
	// However, then it would be harder to write proper admission messages, because we don't even know if finalizers are being modified.
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
	addedFinalizers := changedFinalizers.Intersection(newFinalizers)
	removedFinalizers := changedFinalizers.Intersection(oldFinalizers)

	obj := newObj
	if len(req.Object.Raw) == 0 {
		obj = oldObj
	}

	log := h.Logger.WithValues(
		"requestUID", req.UID,
		"operation", req.Operation,
		"kind", req.Kind.Kind,
		"resource", req.Resource.Resource,
		"namespace", obj.GetNamespace(),
		"name", obj.GetName(),
		"user", req.UserInfo.Username,
		"groups", req.UserInfo.Groups,
		"addedFinalizers", addedFinalizers.UnsortedList(),
		"removedFinalizers", removedFinalizers.UnsortedList(),
	)

	// Apply the gardenletidentity logic (see e.g. pkg/admissioncontroller/webhook/admission/shootrestriction/handler.go:78)
	// Checks various user types in the 'gardener.cloud:system:*' and 'system:serviceaccounts' groups
	_, _, _, userType := shootidentity.FromAuthenticationV1UserInfo(req.UserInfo)
	if userType == gardenletidentity.UserTypeGardenlet {
		log.V(1).Info("Allowing request", "reason", "gardenlet is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("gardenlet is allowed to modify protected finalizer(s) %v", changedFinalizers.UnsortedList()))
	}
	if userType == gardenletidentity.UserTypeExtension {
		log.V(1).Info("Allowing request", "reason", "extension is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("extension is allowed to modify protected finalizer(s) %v", changedFinalizers.UnsortedList()))
	}
	if userType == gardenletidentity.UserTypeGardenadm {
		log.V(1).Info("Allowing request", "reason", "gardenadm is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("gardenadm is allowed to modify protected finalizer(s) %v", changedFinalizers.UnsortedList()))
	}
	_, _, seedUserType := seedidentity.FromAuthenticationV1UserInfo(req.UserInfo)
	if seedUserType == gardenletidentity.UserTypeGardenlet {
		log.V(1).Info("Allowing request", "reason", "gardenlet is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("gardenlet is allowed to modify protected finalizer(s) %v", changedFinalizers.UnsortedList()))
	}
	if seedUserType == gardenletidentity.UserTypeExtension {
		log.V(1).Info("Allowing request", "reason", "extension is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("extension is allowed to modify protected finalizer(s) %v", changedFinalizers.UnsortedList()))
	}

	// Allow all requests from the 'system:serviceaccounts:kube-system' group
	groups := sets.New(req.UserInfo.Groups...)
	if groups.Has(serviceaccount.MakeNamespaceGroupName(metav1.NamespaceSystem)) {
		log.V(1).Info("Allowing request", "reason", "kube-system service account is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("kube-system service account %q is allowed to modify protected finalizer(s) %v", req.UserInfo.Username, changedFinalizers.UnsortedList()))
	}
	// Also allow 'system:masters' group
	if groups.Has(user.SystemPrivilegedGroup) {
		log.V(1).Info("Allowing request", "reason", "cluster administrator is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("cluster administrator user %q is allowed to modify protected finalizer(s) %v", req.UserInfo.Username, changedFinalizers.UnsortedList()))
	}

	// Apply the logic from getAdminUserGroups (pkg/apiserver/registry/core/shoot/storage/admin_kubeconfig.go:54) to check if user is system admin (aka ops)
	// vs just normal project admin by checking if they can list secrets.
	extra := make(map[string]authorizationv1.ExtraValue, len(req.UserInfo.Extra))
	for k, v := range req.UserInfo.Extra {
		extra[k] = authorizationv1.ExtraValue(v)
	}
	systemAccessReview := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:    "",
				Resource: "secrets",
				Verb:     "list",
			},
			User:   req.UserInfo.Username,
			Groups: req.UserInfo.Groups,
			UID:    req.UserInfo.UID,
			Extra:  extra,
		},
	}
	result, err := h.SubjectAccessReviewer.Create(ctx, systemAccessReview, metav1.CreateOptions{})
	if err != nil {
		log.Error(err, "Failed checking whether user is a system administrator")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if result.Status.Allowed {
		log.V(1).Info("Allowing request", "reason", "system administrator is allowed to modify protected finalizers")
		return admission.Allowed(fmt.Sprintf("system administrator user %q is allowed to modify protected finalizer(s) %v", req.UserInfo.Username, changedFinalizers.UnsortedList()))
	}

	log.Info("Denying request", "reason", "user is not allowed to modify the protected finalizers")
	return admission.Denied(fmt.Sprintf("user %q is not allowed to modify protected finalizer(s) %v", req.UserInfo.Username, changedFinalizers.UnsortedList()))
}
