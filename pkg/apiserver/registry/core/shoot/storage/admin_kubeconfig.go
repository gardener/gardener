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

// NewAdminKubeconfigREST returns a new KubeconfigREST for admin kubeconfigs.
func NewAdminKubeconfigREST(
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
		subjectAccessReviewer: subjectAccessReviewer,
		shootStorage:          shootGetter,
		maxExpirationSeconds:  int64(maxExpiration.Seconds()),

		gvk: schema.GroupVersionKind{
			Group:   authenticationv1alpha1.SchemeGroupVersion.Group,
			Version: authenticationv1alpha1.SchemeGroupVersion.Version,
			Kind:    "AdminKubeconfigRequest",
		},
		newObjectFunc: func() runtime.Object {
			return &authenticationv1alpha1.AdminKubeconfigRequest{}
		},
		userGroupsFunc: getAdminUserGroups,
	}
}

func getAdminUserGroups(ctx context.Context, u user.Info, subjectAccessReviewer clientauthorizationv1.SubjectAccessReviewInterface) ([]string, error) {
	subjectAccessReview := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: "",
				Group:     "v1",
				Resource:  "Secret",
				Verb:      "list",
			},
			User:   u.GetName(),
			Groups: u.GetGroups(),
			Extra:  convertToAuthorizationExtraValue(u.GetExtra()),
			UID:    u.GetUID(),
		},
	}

	result, err := subjectAccessReviewer.Create(ctx, subjectAccessReview, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	if result.Status.Allowed {
		return []string{v1beta1constants.GardenerSystemAdminsGroupName}, nil
	}

	return []string{v1beta1constants.GardenerProjectAdminsGroupName}, nil
}
