// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeletionprotection

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ObjectSelector is the object selector for CustomResourceDefinitions used by this admission controller.
var ObjectSelector = map[string]string{gardenerutils.DeletionProtected: "true"}

// Handler validating DELETE requests for extension CRDs and extension resources, that are marked for deletion
// protection (`gardener.cloud/deletion-protected`).
type Handler struct {
	Logger       logr.Logger
	SourceReader client.Reader
	Decoder      admission.Decoder
}

// Handle validates the DELETE request.
func (h *Handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// If the request does not indicate the correct operation (DELETE) we allow the review without further doing.
	if request.Operation != admissionv1.Delete {
		return admission.Allowed("operation is not DELETE")
	}

	var listOp client.ListOption

	// Ignore all resources other than our expected ones
	switch request.Resource {
	case
		metav1.GroupVersionResource{Group: apiextensionsv1beta1.SchemeGroupVersion.Group, Version: apiextensionsv1beta1.SchemeGroupVersion.Version, Resource: "customresourcedefinitions"},
		metav1.GroupVersionResource{Group: apiextensionsv1.SchemeGroupVersion.Group, Version: apiextensionsv1.SchemeGroupVersion.Version, Resource: "customresourcedefinitions"}:
		listOp = client.MatchingLabels(ObjectSelector)

	case
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupbuckets"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupentries"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "containerruntimes"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "controlplanes"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "dnsrecords"},
		metav1.GroupVersionResource{Group: druidcorev1alpha1.SchemeGroupVersion.Group, Version: druidcorev1alpha1.SchemeGroupVersion.Version, Resource: "etcds"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "extensions"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "infrastructures"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "networks"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "operatingsystemconfigs"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "workers"}:
		listOp = client.InNamespace(request.Namespace)

	default:
		return admission.Allowed("resource is not deletion-protected")
	}

	obj, err := ExtractRequestObject(ctx, h.SourceReader, h.Decoder, request, listOp)
	if apierrors.IsNotFound(err) {
		return admission.Allowed("object was not found")
	}
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	operation := "DELETE"
	if strings.HasSuffix(obj.GetObjectKind().GroupVersionKind().Kind, "List") {
		operation = "DELETECOLLECTION"
	}

	log := h.Logger.
		WithValues("resource", request.Resource).
		WithValues("operation", operation).
		WithValues("namespace", request.Namespace)

	log.Info("Handling request")

	if err := admitObjectDeletion(log, obj); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("")
}

// admitObjectDeletion checks if the object deletion is confirmed. If the given object is a list of objects then it
// performs the check for every single object.
func admitObjectDeletion(log logr.Logger, obj runtime.Object) error {
	if strings.HasSuffix(obj.GetObjectKind().GroupVersionKind().Kind, "List") {
		return meta.EachListItem(obj, func(o runtime.Object) error {
			return checkIfObjectDeletionIsConfirmed(log, o)
		})
	}
	return checkIfObjectDeletionIsConfirmed(log, obj)
}

// checkIfObjectDeletionIsConfirmed checks if the object was annotated with the deletion confirmation. If it is a custom
// resource definition then it is only considered if the CRD has the deletion protection label.
func checkIfObjectDeletionIsConfirmed(log logr.Logger, object runtime.Object) error {
	obj, ok := object.(client.Object)
	if !ok {
		return fmt.Errorf("%T does not implement client.Object", object)
	}

	log = log.WithValues("name", obj.GetName())

	if err := gardenerutils.CheckIfDeletionIsConfirmed(obj); err != nil {
		log.Info("Deletion is not confirmed - preventing deletion")
		return err
	}

	log.Info("Deletion is confirmed - allowing deletion")
	return nil
}
