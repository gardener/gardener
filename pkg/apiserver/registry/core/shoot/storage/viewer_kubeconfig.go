// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	authenticationv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
)

// NewViewerKubeconfigREST returns a new KubeconfigREST for viewer kubeconfigs.
func NewViewerKubeconfigREST(
	shootGetter getter,
	secretLister kubecorev1listers.SecretLister,
	internalSecretLister gardencorev1beta1listers.InternalSecretLister,
	configMapLister kubecorev1listers.ConfigMapLister,
	maxExpiration time.Duration,
) *KubeconfigREST {
	return &KubeconfigREST{
		secretLister:         secretLister,
		internalSecretLister: internalSecretLister,
		configMapLister:      configMapLister,
		shootStorage:         shootGetter,
		maxExpirationSeconds: int64(maxExpiration.Seconds()),

		gvk: schema.GroupVersionKind{
			Group:   authenticationv1alpha1.SchemeGroupVersion.Group,
			Version: authenticationv1alpha1.SchemeGroupVersion.Version,
			Kind:    "ViewerKubeconfigRequest",
		},
		newObjectFunc: func() runtime.Object {
			return &authenticationv1alpha1.ViewerKubeconfigRequest{}
		},
		clientCertificateOrganization: v1beta1constants.ShootGroupViewers,
	}
}
