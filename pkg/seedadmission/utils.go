// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
