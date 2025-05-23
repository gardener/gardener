// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeletionprotection

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ExtractRequestObject extracts the object in the admission request and returns it.
// The given `ListOption` is used to list affected objects in case of a `DELETECOLLECTION` request.
func ExtractRequestObject(ctx context.Context, reader client.Reader, decoder admission.Decoder, request admission.Request, listOp client.ListOption) (runtime.Object, error) {
	var (
		obj runtime.Object
		err error
	)

	switch {
	// DELETECOLLECTION requests don't contain the entire object list.
	// Lookup all existing objects of that gvk.
	case request.Name == "":
		o := &unstructured.UnstructuredList{}
		o.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
		o.SetKind(request.Kind.Kind + "List")
		err = reader.List(ctx, o, listOp)
		obj = o

	case request.OldObject.Raw != nil:
		obj = &unstructured.Unstructured{}
		err = decoder.DecodeRaw(request.OldObject, obj)

	case request.Object.Raw != nil:
		obj = &unstructured.Unstructured{}
		err = decoder.DecodeRaw(request.Object, obj)

	default:
		err = fmt.Errorf("no object found in admission request")
	}

	return obj, err
}
