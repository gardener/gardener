// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

//nolint:revive
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/authentication"
)

func Convert_v1alpha1_AdminKubeconfigRequest_To_authentication_KubeconfigRequest(in *AdminKubeconfigRequest, out *authentication.KubeconfigRequest, _ conversion.Scope) error {
	out.Spec.ExpirationSeconds = pointer.Int64Deref(in.Spec.ExpirationSeconds, 0)
	out.Status.Kubeconfig = in.Status.Kubeconfig
	out.Status.ExpirationTimestamp = in.Status.ExpirationTimestamp
	return nil
}

func Convert_authentication_KubeconfigRequest_To_v1alpha1_AdminKubeconfigRequest(in *authentication.KubeconfigRequest, out *AdminKubeconfigRequest, _ conversion.Scope) error {
	out.Spec.ExpirationSeconds = &in.Spec.ExpirationSeconds
	out.Status.Kubeconfig = in.Status.Kubeconfig
	out.Status.ExpirationTimestamp = in.Status.ExpirationTimestamp
	return nil
}

func Convert_v1alpha1_ViewerKubeconfigRequest_To_authentication_KubeconfigRequest(in *ViewerKubeconfigRequest, out *authentication.KubeconfigRequest, _ conversion.Scope) error {
	out.Spec.ExpirationSeconds = pointer.Int64Deref(in.Spec.ExpirationSeconds, 0)
	out.Status.Kubeconfig = in.Status.Kubeconfig
	out.Status.ExpirationTimestamp = in.Status.ExpirationTimestamp
	return nil
}

func Convert_authentication_KubeconfigRequest_To_v1alpha1_ViewerKubeconfigRequest(in *authentication.KubeconfigRequest, out *ViewerKubeconfigRequest, _ conversion.Scope) error {
	out.Spec.ExpirationSeconds = &in.Spec.ExpirationSeconds
	out.Status.Kubeconfig = in.Status.Kubeconfig
	out.Status.ExpirationTimestamp = in.Status.ExpirationTimestamp
	return nil
}
