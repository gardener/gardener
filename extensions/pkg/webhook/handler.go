// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// HandlerBuilder contains information which are required to create an admission handler.
type HandlerBuilder struct {
	actionMap  map[handlerAction][]Type
	predicates []predicate.Predicate
	scheme     *runtime.Scheme
	logger     logr.Logger
}

// NewBuilder creates a new HandlerBuilder.
func NewBuilder(mgr manager.Manager, logger logr.Logger) *HandlerBuilder {
	return &HandlerBuilder{
		actionMap: make(map[handlerAction][]Type),
		scheme:    mgr.GetScheme(),
		logger:    logger.WithName("handler"),
	}
}

// WithMutator adds the given mutator for the given types to the HandlerBuilder.
func (b *HandlerBuilder) WithMutator(mutator Mutator, types ...Type) *HandlerBuilder {
	mutatingAction := mutatingActionHandler(mutator)
	b.actionMap[mutatingAction] = append(b.actionMap[mutatingAction], types...)

	return b
}

// WithValidator adds the given validator for the given types to the HandlerBuilder.
func (b *HandlerBuilder) WithValidator(validator Validator, types ...Type) *HandlerBuilder {
	validatingAction := validatingActionHandler(validator)
	b.actionMap[validatingAction] = append(b.actionMap[validatingAction], types...)

	return b
}

// WithPredicates adds the given predicates to the HandlerBuilder.
func (b *HandlerBuilder) WithPredicates(predicates ...predicate.Predicate) *HandlerBuilder {
	b.predicates = append(b.predicates, predicates...)
	return b
}

// Build creates a new admission.Handler with the settings previously specified with the HandlerBuilder's functions.
func (b *HandlerBuilder) Build() (admission.Handler, error) {
	h := &handler{
		typesMap:   make(map[metav1.GroupVersionKind]client.Object),
		actionMap:  make(map[metav1.GroupVersionKind]handlerAction),
		predicates: b.predicates,
		scheme:     b.scheme,
		logger:     b.logger,
	}

	for m, t := range b.actionMap {
		typesMap, err := buildTypesMap(b.scheme, objectsFromTypes(t))
		if err != nil {
			return nil, err
		}
		mutator := m
		for gvk, obj := range typesMap {
			h.typesMap[gvk] = obj
			h.actionMap[gvk] = mutator
		}
	}
	h.decoder = serializer.NewCodecFactory(b.scheme).UniversalDecoder()

	return h, nil
}

type handlerAction interface {
	do(ctx context.Context, new, old client.Object) error
}

// actionFunc is a func to be used directly as an implementation for handlerAction.
type actionFunc func(ctx context.Context, new, old client.Object) error

func (mf actionFunc) do(ctx context.Context, new, old client.Object) error {
	return mf(ctx, new, old)
}

type handler struct {
	actionMap  map[metav1.GroupVersionKind]handlerAction
	typesMap   map[metav1.GroupVersionKind]client.Object
	predicates []predicate.Predicate
	decoder    runtime.Decoder
	scheme     *runtime.Scheme
	logger     logr.Logger
}

// Handle handles the given admission request.
func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	ar := req.AdmissionRequest

	// Decode object
	t, ok := h.typesMap[ar.Kind]
	if !ok {
		// check if we can find an internal type
		for gvk, obj := range h.typesMap {
			if gvk.Version == runtime.APIVersionInternal && gvk.Group == ar.Kind.Group && gvk.Kind == ar.Kind.Kind {
				t = obj
				break
			}
		}
		if t == nil {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected request kind %s", ar.Kind.String()))
		}
	}

	mutator, ok := h.actionMap[ar.Kind]
	if !ok {
		// check if we can find an internal type
		for gvk, m := range h.actionMap {
			if gvk.Version == runtime.APIVersionInternal && gvk.Group == ar.Kind.Group && gvk.Kind == ar.Kind.Kind {
				mutator = m
				break
			}
		}
		if mutator == nil {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected request kind %s", ar.Kind.String()))
		}
	}

	return handle(ctx, req, mutator, t, h.decoder, h.logger, h.predicates...)
}

func handle(ctx context.Context, req admission.Request, m handlerAction, t client.Object, decoder runtime.Decoder, logger logr.Logger, predicates ...predicate.Predicate) admission.Response {
	ar := req.AdmissionRequest

	// Decode object
	obj := t.DeepCopyObject().(client.Object)
	_, _, err := decoder.Decode(req.Object.Raw, nil, obj)
	if err != nil {
		logger.Error(err, "Could not decode request", "request", ar)
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode request %v: %w", ar, err))
	}

	var oldObj client.Object

	// Only UPDATE and DELETE operations have oldObjects.
	if len(req.OldObject.Raw) != 0 {
		oldObj = t.DeepCopyObject().(client.Object)
		if _, _, err := decoder.Decode(ar.OldObject.Raw, nil, oldObj); err != nil {
			logger.Error(err, "Could not decode old object", "object", oldObj)
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode old object %v: %v", oldObj, err))
		}
	}

	// Run object through predicates
	if !predicateutils.EvalGeneric(obj, predicates...) {
		return admission.ValidationResponse(true, "")
	}

	// Process the resource
	newObj := obj.DeepCopyObject().(client.Object)
	if err = m.do(ctx, newObj, oldObj); err != nil {
		logger.Error(fmt.Errorf("could not process: %w", err), "Admission denied", "kind", ar.Kind.Kind, "namespace", obj.GetNamespace(), "name", obj.GetName())
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	_, isValidator := m.(Validator)
	// Return a patch response if the resource should be changed
	if !isValidator && !equality.Semantic.DeepEqual(obj, newObj) {
		oldObjMarshaled, err := json.Marshal(obj)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		newObjMarshaled, err := json.Marshal(newObj)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return admission.PatchResponseFromRaw(oldObjMarshaled, newObjMarshaled)
	}

	// Return a validation response if the resource should not be changed
	return admission.ValidationResponse(true, "")
}

func objectsFromTypes(in []Type) []client.Object {
	out := make([]client.Object, 0, len(in))
	for _, t := range in {
		out = append(out, t.Obj)
	}
	return out
}

// buildTypesMap builds a map of the given types keyed by their GroupVersionKind, using the scheme from the given Manager.
func buildTypesMap(scheme *runtime.Scheme, types []client.Object) (map[metav1.GroupVersionKind]client.Object, error) {
	typesMap := make(map[metav1.GroupVersionKind]client.Object)
	for _, t := range types {
		// Get GVK from the type
		gvk, err := apiutil.GVKForObject(t, scheme)
		if err != nil {
			return nil, fmt.Errorf("could not get GroupVersionKind from object %v: %w", t, err)
		}

		// Add the type to the types map
		typesMap[metav1.GroupVersionKind(gvk)] = t
	}
	return typesMap, nil
}
