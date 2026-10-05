// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientauthorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	authenticationv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// NewViewerKubeconfigREST returns a new KubeconfigREST for viewer kubeconfigs.
func NewViewerKubeconfigREST(
	shootGetter getter,
	secretLister kubecorev1listers.SecretLister,
	internalSecretLister gardencorev1beta1listers.InternalSecretLister,
	configMapLister kubecorev1listers.ConfigMapLister,
	maxExpiration time.Duration,
	subjectAccessReviewer clientauthorizationv1.SubjectAccessReviewInterface,
) *KubeconfigREST {
	return &KubeconfigREST{
		secretLister:          secretLister,
		internalSecretLister:  internalSecretLister,
		configMapLister:       configMapLister,
		shootStorage:          shootGetter,
		maxExpirationSeconds:  int64(maxExpiration.Seconds()),
		subjectAccessReviewer: subjectAccessReviewer,

		gvk: schema.GroupVersionKind{
			Group:   authenticationv1alpha1.SchemeGroupVersion.Group,
			Version: authenticationv1alpha1.SchemeGroupVersion.Version,
			Kind:    "ViewerKubeconfigRequest",
		},
		newObjectFunc: func() runtime.Object {
			return &authenticationv1alpha1.ViewerKubeconfigRequest{}
		},
		userGroupsFunc: gardenerutils.GetViewerUserGroups,
		userNamePrefix: "gardener.cloud:viewer:",
	}
}
