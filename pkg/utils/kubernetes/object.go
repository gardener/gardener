// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Object is wrapper interface combining runtime.Object and metav1.Object interfaces together.
type Object interface {
	runtime.Object
	metav1.Object
}

// ObjectName returns the name of the given object in the format <namespace>/<name>
func ObjectName(obj runtime.Object) string {
	k, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return "/"
	}
	return k.String()
}
