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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"

	"k8s.io/api/admission/v1beta1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type namespaceDeletionHandler struct {
	k8sGardenClient kubernetes.Interface

	projectLister              gardenlisters.ProjectLister
	backupInfrastructureLister gardenlisters.BackupInfrastructureLister
	shootLister                gardenlisters.ShootLister

	scheme *runtime.Scheme
	codecs serializer.CodecFactory
}

// NewValidateNamespaceDeletionHandler creates a new handler for validating namespace deletions.
func NewValidateNamespaceDeletionHandler(k8sGardenClient kubernetes.Interface, projectLister gardenlisters.ProjectLister, backupInfrastructureLister gardenlisters.BackupInfrastructureLister, shootLister gardenlisters.ShootLister) func(http.ResponseWriter, *http.Request) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	admissionregistrationv1beta1.AddToScheme(scheme)

	h := &namespaceDeletionHandler{k8sGardenClient, projectLister, backupInfrastructureLister, shootLister, scheme, serializer.NewCodecFactory(scheme)}
	return h.ValidateNamespaceDeletion
}

// ValidateNamespaceDeletion is a HTTP handler for validating whether a namespace deletion is allowed or not.
func (h *namespaceDeletionHandler) ValidateNamespaceDeletion(w http.ResponseWriter, r *http.Request) {
	var (
		body []byte

		deserializer   = h.codecs.UniversalDeserializer()
		receivedReview = v1beta1.AdmissionReview{}

		wantedContentType = "application/json"
		wantedOperation   = admissionv1beta1.Delete
	)

	// Read HTTP request body into variable.
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// Verify that the correct content-type header has been sent.
	if contentType := r.Header.Get("Content-Type"); contentType != wantedContentType {
		err := fmt.Errorf("contentType=%s, expect %s", contentType, wantedContentType)
		logger.Logger.Errorf(err.Error())
		respond(w, errToAdmissionResponse(err))
		return
	}

	// Deserialize HTTP request body into admissionv1beta1.AdmissionReview object.
	if _, _, err := deserializer.Decode(body, nil, &receivedReview); err != nil {
		logger.Logger.Errorf(err.Error())
		respond(w, errToAdmissionResponse(err))
		return
	}

	// If the request field is empty then do not admit (invalid body).
	if receivedReview.Request == nil {
		err := fmt.Errorf("invalid request body (missing admission request)")
		logger.Logger.Errorf(err.Error())
		respond(w, errToAdmissionResponse(err))
		return
	}
	// If the request does not indicate the correct operation (DELETE) we allow the review without further doing.
	if receivedReview.Request.Operation != wantedOperation {
		respond(w, admissionResponse(true, ""))
		return
	}

	// Now that all checks have been passed we can actually validate the admission request.
	reviewResponse := h.admitNamespaces(receivedReview.Request)
	if !reviewResponse.Allowed && reviewResponse.Result != nil {
		logger.Logger.Infof("Rejected 'DELETE namespace' request of user '%s': %v", receivedReview.Request.UserInfo.Username, reviewResponse.Result.Message)
	}
	respond(w, reviewResponse)
}

// admitNamespaces does only allow the request if no Shoots and no BackupInfrastructures exist in this
// specific namespace anymore.
func (h *namespaceDeletionHandler) admitNamespaces(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	namespaceResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	if request.Resource != namespaceResource {
		return errToAdmissionResponse(fmt.Errorf("expect resource to be %s", namespaceResource))
	}

	// Determine project object for given namespace.
	project, err := common.ProjectForNamespace(h.projectLister, request.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Namespace does not belong to a project. Deletion is allowed.
			return admissionResponse(true, "")
		}
		return errToAdmissionResponse(err)
	}

	// We do not receive the namespace object in the `.object` field of the admission request. Hence, we need to get it ourselves.
	namespace, err := h.k8sGardenClient.GetNamespace(request.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return admissionResponse(true, "")
		}
		return errToAdmissionResponse(err)
	}

	switch {
	case namespace.DeletionTimestamp != nil:
		// Namespace is already marked to be deleted so we can allow the request.
		return admissionResponse(true, "")
	case project.DeletionTimestamp != nil:
		namespaceEmpty, err := h.isNamespaceEmpty(namespace.Name)
		if err != nil {
			return errToAdmissionResponse(err)
		}

		if namespaceEmpty {
			return admissionResponse(true, "")
		}
		return admissionResponse(false, fmt.Sprintf("Deletion of namespace %q is not permitted (there are still Shoots or BackupInfrastructures).", namespace.Name))
	}

	// Namespace is not yet marked to be deleted and project is not marked as well. We do not admit and respond that namespace deletion is only
	// allowed via project deletion.
	return admissionResponse(false, fmt.Sprintf("Direct deletion of namespace %q is not permitted (you must delete the corresponding project %q).", namespace.Name, project.Name))
}

func (h *namespaceDeletionHandler) isNamespaceEmpty(namespace string) (bool, error) {
	backupInfrastructureList, err := h.backupInfrastructureLister.BackupInfrastructures(namespace).List(labels.Everything())
	if err != nil {
		return false, err
	}

	if len(backupInfrastructureList) != 0 {
		return false, nil
	}

	shootList, err := h.shootLister.Shoots(namespace).List(labels.Everything())
	if err != nil {
		return false, err
	}

	return len(shootList) == 0, nil
}

func respond(w http.ResponseWriter, response *v1beta1.AdmissionResponse) {
	responseObj := v1beta1.AdmissionReview{}
	if response != nil {
		responseObj.Response = response
	}

	jsonResponse, err := json.Marshal(responseObj)
	if err != nil {
		logger.Logger.Error(err)
	}
	if _, err := w.Write(jsonResponse); err != nil {
		logger.Logger.Error(err)
	}
}
