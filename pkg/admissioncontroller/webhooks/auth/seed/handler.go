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

package seed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	authorizationv1 "k8s.io/api/authorization/v1"
	authorizationv1beta1 "k8s.io/api/authorization/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
)

// WebhookPath is the HTTP handler path for this authorization webhook handler.
const WebhookPath = "/webhooks/auth/seed"

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)

	// DecisionTimeout is the maximum time for the authorizer to take a decision. Exposed for testing.
	DecisionTimeout = 10 * time.Second
)

func init() {
	utilruntime.Must(authorizationv1beta1.AddToScheme(scheme))
	utilruntime.Must(authorizationv1.AddToScheme(scheme))
}

// NewHandler creates a new HTTP handler for authorizing requests for resources related to a Seed.
func NewHandler(logger logr.Logger, authorizer auth.Authorizer) http.HandlerFunc {
	h := &handler{
		logger:     logger,
		authorizer: authorizer,
	}
	return h.Handle
}

type handler struct {
	logger     logr.Logger
	authorizer auth.Authorizer
}

func (h *handler) Handle(w http.ResponseWriter, r *http.Request) {
	var (
		ctx, cancel = context.WithTimeout(r.Context(), DecisionTimeout)
		body        []byte
		err         error
	)
	defer cancel()

	// Verify that body is non-empty
	if r.Body == nil {
		err = errors.New("request body is empty")
		h.logger.Error(err, "bad request")
		h.writeResponse(w, nil, Errored(http.StatusBadRequest, err))
		return
	}

	// Read body
	if body, err = io.ReadAll(r.Body); err != nil {
		h.logger.Error(err, "unable to read the body from the incoming request")
		h.writeResponse(w, nil, Errored(http.StatusBadRequest, err))
		return
	}

	// Verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err = fmt.Errorf("contentType=%s, expected application/json", contentType)
		h.logger.Error(err, "unable to process a request with an unknown content type", "content type", contentType)
		h.writeResponse(w, nil, Errored(http.StatusBadRequest, err))
		return
	}

	// Decode request body into authorizationv1beta1.SubjectAccessReviewSpec structure
	sarSpec, gvk, err := h.decodeRequestBody(body)
	if err != nil {
		h.logger.Error(err, "unable to decode the request")
		h.writeResponse(w, nil, Errored(http.StatusBadRequest, err))
		return
	}

	// Log request information
	keysAndValues := []interface{}{"user", sarSpec.User, "groups", sarSpec.Groups}
	if sarSpec.ResourceAttributes != nil {
		keysAndValues = append(keysAndValues, "resourceAttributes", sarSpec.ResourceAttributes.String())
	}
	if sarSpec.NonResourceAttributes != nil {
		keysAndValues = append(keysAndValues, "nonResourceAttributes", sarSpec.NonResourceAttributes.String())
	}
	h.logger.V(1).Info("received request", keysAndValues...)

	// Consult authorizer for result and write the response
	decision, reason, err := h.authorizer.Authorize(ctx, AuthorizationAttributesFrom(*sarSpec))
	if err != nil {
		h.logger.Error(err, "error when consulting authorizer for opinion")
		h.writeResponse(w, gvk, Errored(http.StatusInternalServerError, err))
		return
	}

	var status authorizationv1.SubjectAccessReviewStatus
	switch decision {
	case auth.DecisionAllow:
		status = Allowed()
	case auth.DecisionDeny:
		status = Denied(reason)
	case auth.DecisionNoOpinion:
		status = NoOpinion(reason)
	default:
		status = Errored(http.StatusInternalServerError, fmt.Errorf("unexpected decision: %d", decision))
	}

	h.logger.V(1).Info("responding to request", append([]interface{}{"decision", decision, "reason", reason}, keysAndValues...))
	h.writeResponse(w, gvk, status)
}

func (h *handler) writeResponse(w io.Writer, gvk *schema.GroupVersionKind, status authorizationv1.SubjectAccessReviewStatus) {
	// The SubjectAccessReviewStatus type is exactly the same for both authorization.k8s.io/v1beta1 and
	// authorization.k8s.io/v1, so we can just overwrite the apiVersion/kind with the same version like the object in
	// the request.
	response := authorizationv1.SubjectAccessReview{Status: status}
	if gvk == nil || *gvk == (schema.GroupVersionKind{}) {
		response.SetGroupVersionKind(authorizationv1.SchemeGroupVersion.WithKind("SubjectAccessReview"))
	} else {
		response.SetGroupVersionKind(*gvk)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error(err, "unable to encode the response")
		h.writeResponse(w, gvk, Errored(http.StatusInternalServerError, err))
		return
	}

	if log := h.logger; log.V(1).Enabled() {
		log = log.WithValues(
			"allowed", status.Allowed,
			"denied", status.Denied,
			"reason", status.Reason,
			"error", status.EvaluationError,
		)
		log.V(1).Info("wrote response")
	}
}

func (h *handler) decodeRequestBody(body []byte) (*authorizationv1.SubjectAccessReviewSpec, *schema.GroupVersionKind, error) {
	// v1 and v1beta1 SubjectAccessReview types are almost exactly the same (the only difference is the JSON key for the
	// 'Groups' field). We decode the object into a v1 type and "manually" convert the 'Groups' field (see below).
	// However, the runtime codec's decoder guesses which type to decode into by type name if an Object's TypeMeta
	// isn't set. By setting TypeMeta of an unregistered type to the v1 GVK, the decoder will coerce a v1beta1
	// SubjectAccessReview to v1.
	var obj unversionedAdmissionReview
	obj.SetGroupVersionKind(authorizationv1.SchemeGroupVersion.WithKind("SubjectAccessReview"))

	_, gvk, err := codecs.UniversalDeserializer().Decode(body, nil, &obj)
	if err != nil {
		return nil, nil, err
	}
	if gvk == nil {
		return nil, nil, fmt.Errorf("could not determine GVK for object in the request body")
	}

	// The only difference in v1beta1 is that the JSON key name of the 'Groups' field is different. Hence, when we
	// detect that v1beta1 was sent, we decode it once again into the "correct" type and manually "convert" the 'Groups'
	// information.
	switch *gvk {
	case authorizationv1beta1.SchemeGroupVersion.WithKind("SubjectAccessReview"):
		var tmp authorizationv1beta1.SubjectAccessReview
		if _, _, err := codecs.UniversalDeserializer().Decode(body, nil, &tmp); err != nil {
			return nil, nil, err
		}
		obj.Spec.Groups = tmp.Spec.Groups
	}

	return &obj.Spec, gvk, nil
}

type unversionedAdmissionReview struct {
	authorizationv1.SubjectAccessReview
}
