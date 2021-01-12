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

package seedadmission

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
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
	"github.com/gardener/gardener/pkg/operation/common"
)

// NewExtensionDeletionProtection creates a new handler validating DELETE requests for extension CRDs and extension
// resources, that are marked for deletion protection (`gardener.cloud/deletion-protected`).
// TODO: remove this constructor in favor of InjectLogger once we have switched to logr.
func NewExtensionDeletionProtection(logger logrus.FieldLogger) admission.Handler {
	return &extensionDeletionProtection{
		logger: logger,
	}
}

type extensionDeletionProtection struct {
	logger  logrus.FieldLogger
	client  client.Client
	decoder *admission.Decoder
}

// InjectClient injects a client.
func (h *extensionDeletionProtection) InjectClient(c client.Client) error {
	h.client = c
	return nil
}

// InjectDecoder injects a decoder capable of decoding objects included in admission requests.
func (h *extensionDeletionProtection) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// Handle implements the webhook handler for extension deletion protection.
func (h *extensionDeletionProtection) Handle(ctx context.Context, request admission.Request) admission.Response {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// If the request does not indicate the correct operation (DELETE) we allow the review without further doing.
	if request.Operation != admissionv1.Delete {
		return admission.Allowed("operation is not DELETE")
	}

	// Ignore all resources other than our expected ones
	switch request.Resource {
	case
		metav1.GroupVersionResource{Group: apiextensionsv1beta1.SchemeGroupVersion.Group, Version: apiextensionsv1beta1.SchemeGroupVersion.Version, Resource: "customresourcedefinitions"},
		metav1.GroupVersionResource{Group: apiextensionsv1.SchemeGroupVersion.Group, Version: apiextensionsv1.SchemeGroupVersion.Version, Resource: "customresourcedefinitions"},

		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupbuckets"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupentries"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "containerruntimes"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "controlplanes"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "extensions"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "infrastructures"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "networks"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "operatingsystemconfigs"},
		metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "workers"}:
	default:
		return admission.Allowed("resource is not deletion-protected")
	}

	obj, err := getRequestObject(ctx, h.client, h.decoder, request)
	if apierrors.IsNotFound(err) {
		return admission.Allowed("object was not found")
	}
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	var operation string
	if strings.HasSuffix(obj.GetObjectKind().GroupVersionKind().Kind, "List") {
		operation = "DELETECOLLECTION"
	} else {
		operation = "DELETE"
	}

	entryLogger := h.logger.
		WithField("resource", fmt.Sprintf("%s/%s/%s", request.Kind.Group, request.Kind.Version, request.Kind.Kind)).
		WithField("operation", operation).
		WithField("namespace", request.Namespace)

	entryLogger.Info("Handling request")

	if err := admitObjectDeletion(entryLogger, obj, request.Kind.Kind); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("")
}

// admitObjectDeletion checks if the object deletion is confirmed. If the given object is a list of objects then it
// performs the check for every single object.
func admitObjectDeletion(logger logrus.FieldLogger, obj runtime.Object, kind string) error {
	if strings.HasSuffix(obj.GetObjectKind().GroupVersionKind().Kind, "List") {
		return meta.EachListItem(obj, func(o runtime.Object) error {
			return checkIfObjectDeletionIsConfirmed(logger, o, kind)
		})
	}
	return checkIfObjectDeletionIsConfirmed(logger, obj, kind)
}

// checkIfObjectDeletionIsConfirmed checks if the object was annotated with the deletion confirmation. If it is a custom
// resource definition then it is only considered if the CRD has the deletion protection label.
func checkIfObjectDeletionIsConfirmed(logger logrus.FieldLogger, obj runtime.Object, kind string) error {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	logger = logger.WithField("name", acc.GetName())

	if kind == "CustomResourceDefinition" && !crdMustBeConsidered(logger, acc.GetLabels()) {
		return nil
	}

	if err := common.CheckIfDeletionIsConfirmed(acc); err != nil {
		logger.Info("Deletion is not confirmed - preventing deletion")
		return err
	}

	logger.Info("Deletion is confirmed - allowing deletion")
	return nil
}

// TODO: This function can be removed once the minimum seed Kubernetes version is bumped to >= 1.15. In 1.15, webhook
// configurations may use object selectors, i.e., we can get rid of this custom filtering.
func crdMustBeConsidered(logger logrus.FieldLogger, labels map[string]string) bool {
	val, ok := labels[common.GardenerDeletionProtected]
	if !ok {
		logger.Infof("Ignoring CRD because it has no %s label - allowing deletion", common.GardenerDeletionProtected)
		return false
	}

	if ok, _ := strconv.ParseBool(val); !ok {
		logger.Infof("Admitting CRD deletion because %s label value is not true - allowing deletion", common.GardenerDeletionProtected)
		return false
	}

	return true
}
