// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	clientauthorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
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
		userGroupsFunc: getViewerUserGroups,
	}
}

func getViewerUserGroups(ctx context.Context, u user.Info, subjectAccessReviewer clientauthorizationv1.SubjectAccessReviewInterface) ([]string, error) {
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: "",
				Group:     "core.gardener.cloud/v1beta1",
				Resource:  "Project",
				Verb:      "list",
			},
			User:   u.GetName(),
			Groups: u.GetGroups(),
			Extra:  convertToAuthorizationExtraValue(u.GetExtra()),
			UID:    u.GetUID(),
		},
	}

	result, err := subjectAccessReviewer.Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	if result.Status.Allowed {
		return []string{v1beta1constants.ShootSystemViewersGroupName}, nil
	}

	return []string{v1beta1constants.ShootProjectViewersGroupName}, nil
}
