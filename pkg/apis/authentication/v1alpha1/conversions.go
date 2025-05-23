// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//nolint:revive
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/authentication"
)

func Convert_v1alpha1_AdminKubeconfigRequest_To_authentication_KubeconfigRequest(in *AdminKubeconfigRequest, out *authentication.KubeconfigRequest, _ conversion.Scope) error {
	out.Spec.ExpirationSeconds = ptr.Deref(in.Spec.ExpirationSeconds, 0)
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
	out.Spec.ExpirationSeconds = ptr.Deref(in.Spec.ExpirationSeconds, 0)
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
