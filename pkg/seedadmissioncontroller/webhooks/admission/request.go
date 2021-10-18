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

package admission

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
func ExtractRequestObject(ctx context.Context, reader client.Reader, decoder *admission.Decoder, request admission.Request, listOp client.ListOption) (runtime.Object, error) {
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
