// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

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

// Errored constructs a SubjectAccessReview and indicates in its status that the an error has been occurred during the
// evaluation of the result.
func Errored(code int32, err error) authorizationv1.SubjectAccessReviewStatus {
	return authorizationv1.SubjectAccessReviewStatus{
		EvaluationError: fmt.Sprintf("%d %s", code, err),
	}
}
