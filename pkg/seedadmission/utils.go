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
	"encoding/json"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getRequestObject(ctx context.Context, c client.Client, request *admissionv1beta1.AdmissionRequest) (runtime.Object, error) {
	// Older Kubernetes versions don't provide the object neither in OldObject nor in the Object field. In this case
	// we have to look it up ourselves.
	var (
		obj runtime.Object
		err error
	)

	switch {
	case request.OldObject.Raw != nil:
		o := &unstructured.Unstructured{}
		err = json.Unmarshal(request.OldObject.Raw, o)
		obj = o

	case request.Object.Raw != nil:
		o := &unstructured.Unstructured{}
		err = json.Unmarshal(request.Object.Raw, o)
		obj = o

	case request.Name == "":
		o := &unstructured.UnstructuredList{}
		o.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
		o.SetKind(request.Kind.Kind + "List")
		err = c.List(ctx, o, client.InNamespace(request.Namespace))
		obj = o

	default:
		o := &unstructured.Unstructured{}
		o.SetAPIVersion(request.Kind.Group + "/" + request.Kind.Version)
		o.SetKind(request.Kind.Kind)
		err = c.Get(ctx, kutil.Key(request.Namespace, request.Name), o)
		obj = o
	}

	return obj, err
}
