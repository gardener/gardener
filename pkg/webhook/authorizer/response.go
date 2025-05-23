// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authorizer

import (
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
)

// Allowed constructs a SubjectAccessReview and indicates in its status that the given operation is allowed.
func Allowed() authorizationv1.SubjectAccessReviewStatus {
	return authorizationv1.SubjectAccessReviewStatus{
		Allowed: true,
	}
}

// Denied constructs a SubjectAccessReview and indicates in its status that the given operation is denied and that
// other authenticators should not be consulted for their opinion.
func Denied(reason string) authorizationv1.SubjectAccessReviewStatus {
	return authorizationv1.SubjectAccessReviewStatus{
		Allowed: false,
		Denied:  true,
		Reason:  reason,
	}
}

// NoOpinion constructs a SubjectAccessReview and indicates in its status that the authorizer does not have an opinion
// about the result, i.e., other authenticators should be consulted for their opinion.
func NoOpinion(reason string) authorizationv1.SubjectAccessReviewStatus {
	return authorizationv1.SubjectAccessReviewStatus{
		Allowed: false,
		Reason:  reason,
	}
}

// Errored constructs a SubjectAccessReview and indicates in its status that an error has occurred during the
// evaluation of the result.
func Errored(code int32, err error) authorizationv1.SubjectAccessReviewStatus {
	return authorizationv1.SubjectAccessReviewStatus{
		EvaluationError: fmt.Sprintf("%d %s", code, err),
	}
}
