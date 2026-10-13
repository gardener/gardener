// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	clientauthorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GetViewerUserGroups returns "gardener.cloud:system:viewers" if the user has permissions to list projects, otherwise returns "gardener.cloud:project:viewers".
func GetViewerUserGroups(ctx context.Context, u user.Info, subjectAccessReviewer clientauthorizationv1.SubjectAccessReviewInterface) ([]string, error) {
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: "",
				Group:     gardencorev1beta1.SchemeGroupVersion.Group,
				Resource:  "projects",
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

// GetAdminUserGroups returns "gardener.cloud:system:admins" if the user has permissions to list secrets, otherwise returns "gardener.cloud:project:admins".
func GetAdminUserGroups(ctx context.Context, u user.Info, subjectAccessReviewer clientauthorizationv1.SubjectAccessReviewInterface) ([]string, error) {
	subjectAccessReview := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: "",
				Group:     corev1.SchemeGroupVersion.Group,
				Resource:  "secrets",
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
		return []string{v1beta1constants.ShootSystemAdminsGroupName}, nil
	}

	return []string{v1beta1constants.ShootProjectAdminsGroupName}, nil
}

func convertToAuthorizationExtraValue(extra map[string][]string) map[string]authorizationv1.ExtraValue {
	if extra == nil {
		return nil
	}
	ret := make(map[string]authorizationv1.ExtraValue, len(extra))
	for k, v := range extra {
		ret[k] = v
	}
	return ret
}
