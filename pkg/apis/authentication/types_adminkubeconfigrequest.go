/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package authentication

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AdminKubeconfigRequest can be used to request a kubeconfig with admin credentials
// for a Shoot cluster.
type AdminKubeconfigRequest struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec is the specification of the AdminKubeconfigRequest.
	Spec AdminKubeconfigRequestSpec
	// Status is the status of the AdminKubeconfigRequest.
	Status AdminKubeconfigRequestStatus
}

// AdminKubeconfigRequestStatus is the status of the AdminKubeconfigRequest containing
// the kubeconfig and expiration of the credential.
type AdminKubeconfigRequestStatus struct {
	// Kubeconfig contains the kubeconfig with cluster-admin privileges for the shoot cluster.
	Kubeconfig []byte
	// ExpirationTimestamp is the expiration timestamp of of the returned credential.
	ExpirationTimestamp metav1.Time
}

// AdminKubeconfigRequestSpec contains the expiration time of the kubeconfig.
type AdminKubeconfigRequestSpec struct {
	// ExpirationSeconds is the requested validity duration of the credential. The
	// credential issuer may return a credential with a different validity duration so a
	// client needs to check the 'expirationTimestamp' field in a response.
	// Defaults to 1 hour.
	ExpirationSeconds int64
}
