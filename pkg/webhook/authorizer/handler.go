// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authorizer

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

// Handler authorizing requests for resources.
type Handler struct {
	Logger     logr.Logger
	Authorizer auth.Authorizer
}

// ServeHTTP authorizing requests for resources.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			for _, fn := range utilruntime.PanicHandlers {
				fn(nil, r)
			}
			h.writeResponse(w, nil, Errored(http.StatusInternalServerError, fmt.Errorf("panic: %v [recovered]", r)))
			return
		}
	}()

	var (
		ctx, cancel = context.WithTimeout(r.Context(), DecisionTimeout)
		body        []byte
		err         error
	)

	defer cancel()

	// Verify that body is non-empty
	if r.Body == nil {
		err = errors.New("request body is empty")
		h.Logger.Error(err, "Bad request")
		h.writeResponse(w, nil, Errored(http.StatusUnprocessableEntity, err))
		return
	}

	// Read body
	if body, err = io.ReadAll(r.Body); err != nil {
		h.Logger.Error(err, "Unable to read the body from the incoming request")
		h.writeResponse(w, nil, Errored(http.StatusBadRequest, err))
		return
	}

	// Verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err = fmt.Errorf("contentType=%s, expected application/json", contentType)
		h.Logger.Error(err, "Unable to process a request with an unknown content type", "contentType", contentType)
		h.writeResponse(w, nil, Errored(http.StatusBadRequest, err))
		return
	}

	// Decode request body into authorizationv1beta1.SubjectAccessReviewSpec structure
	sarSpec, gvk, err := h.decodeRequestBody(body)
	if err != nil {
		h.Logger.Error(err, "Unable to decode the request")
		h.writeResponse(w, nil, Errored(http.StatusBadRequest, err))
		return
	}

	// Log request information
	log := h.Logger.WithValues("user", sarSpec.User, "groups", sarSpec.Groups,
		"resourceAttributes", sarSpec.ResourceAttributes.String(), "nonResourceAttributes", sarSpec.NonResourceAttributes.String())
	log.V(1).Info("Received request")

	// Consult authorizer for result and write the response
	decision, reason, err := h.Authorizer.Authorize(ctx, AuthorizationAttributesFrom(*sarSpec))
	if err != nil {
		log.Error(err, "Error when consulting authorizer for opinion")
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

	log.V(1).Info("Responding to request", "decision", decision, "reason", reason)
	h.writeResponse(w, gvk, status)
}

func (h *Handler) writeResponse(w io.Writer, gvk *schema.GroupVersionKind, status authorizationv1.SubjectAccessReviewStatus) {
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
		h.Logger.Error(err, "Unable to encode the response")
		h.writeResponse(w, gvk, Errored(http.StatusInternalServerError, err))
		return
	}

	if log := h.Logger; log.V(1).Enabled() {
		log = log.WithValues(
			"allowed", status.Allowed,
			"denied", status.Denied,
			"reason", status.Reason,
			"error", status.EvaluationError,
		)
		log.V(1).Info("Wrote response")
	}
}

func (h *Handler) decodeRequestBody(body []byte) (*authorizationv1.SubjectAccessReviewSpec, *schema.GroupVersionKind, error) {
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
		return nil, nil, errors.New("could not determine GVK for object in the request body")
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
