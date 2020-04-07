// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// NewHandler creates a new handler for the given types, using the given mutator, and logger.
func NewHandler(mgr manager.Manager, types []runtime.Object, mutator Mutator, logger logr.Logger) (*handler, error) {
	// Build a map of the given types keyed by their GVKs
	typesMap, err := buildTypesMap(mgr, types)
	if err != nil {
		return nil, err
	}

	// Create and return a handler
	return &handler{
		typesMap: typesMap,
		mutator:  mutator,
		logger:   logger.WithName("handler"),
	}, nil
}

type handler struct {
	typesMap map[metav1.GroupVersionKind]runtime.Object
	mutator  Mutator
	decoder  *admission.Decoder
	logger   logr.Logger
}

// InjectDecoder injects the given decoder into the handler.
func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// InjectClient injects the given client into the mutator.
// TODO Replace this with the more generic InjectFunc when controller runtime supports it
func (h *handler) InjectClient(client client.Client) error {
	if _, err := inject.ClientInto(client, h.mutator); err != nil {
		return errors.Wrap(err, "could not inject the client into the mutator")
	}
	return nil
}

// Handle handles the given admission request.
func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	f := func(ctx context.Context, new, old runtime.Object, r *http.Request) error {
		return h.mutator.Mutate(ctx, new, old)
	}
	return handle(ctx, req, nil, f, h.typesMap, h.decoder, h.logger)
}

type mutateFunc func(ctx context.Context, new, old runtime.Object, r *http.Request) error

func handle(ctx context.Context, req admission.Request, r *http.Request, f mutateFunc, typesMap map[metav1.GroupVersionKind]runtime.Object, decoder *admission.Decoder, logger logr.Logger) admission.Response {
	ar := req.AdmissionRequest

	// Decode object
	t, ok := typesMap[ar.Kind]
	if !ok {
		return admission.Errored(http.StatusBadRequest, errors.Errorf("unexpected request kind %s", ar.Kind.String()))
	}
	obj := t.DeepCopyObject()
	err := decoder.Decode(req, obj)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, errors.Wrapf(err, "could not decode request %v", ar))
	}

	// Get object accessor
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, errors.Wrapf(err, "could not get accessor for %v", obj))
	}

	var oldObj runtime.Object

	// Only UPDATE and DELETE operations have oldObjects.
	if len(req.OldObject.Raw) != 0 {
		oldObj = t.DeepCopyObject()
		if err := decoder.DecodeRaw(ar.OldObject, oldObj); err != nil {
			return admission.Errored(http.StatusBadRequest, errors.Wrapf(err, "could not decode old object %v", oldObj))
		}
	}

	// Mutate the resource
	newObj := obj.DeepCopyObject()
	if err = f(ctx, newObj, oldObj, r); err != nil {
		return admission.Errored(http.StatusInternalServerError,
			errors.Wrapf(err, "could not mutate %s %s/%s", ar.Kind.Kind, accessor.GetNamespace(), accessor.GetName()))
	}

	// Return a patch response if the resource should be changed
	if !equality.Semantic.DeepEqual(obj, newObj) {
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
