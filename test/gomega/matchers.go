// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gomega

import (
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// DeepEqual returns a Gomega matcher which checks whether the expected object is deeply equal with the object it is
// being compared against.
func DeepEqual(expected interface{}) types.GomegaMatcher {
	// if CharactersAroundMismatchToInclude is too small, then format.MessageWithDiff will be unable to output our
	// mismatch message
	if format.CharactersAroundMismatchToInclude < 50 {
		format.CharactersAroundMismatchToInclude = 50
	}

	return newDeepEqualMatcher(expected)
}

// DeepDerivativeEqual is similar to DeepEqual except that unset fields in actual are
// ignored (not compared). This allows us to focus on the fields that matter to
// the semantic comparison.
func DeepDerivativeEqual(expected interface{}) types.GomegaMatcher {
	// if CharactersAroundMismatchToInclude is too small, then format.MessageWithDiff will be unable to output our
	// mismatch message
	if format.CharactersAroundMismatchToInclude < 50 {
		format.CharactersAroundMismatchToInclude = 50
	}

	return newDeepDerivativeMatcher(expected)
}

// BeNotFoundError checks if error is NotFound.
func BeNotFoundError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsNotFound,
		message:   "NotFound",
	}
}

// BeForbiddenError checks if error is Forbidden.
func BeForbiddenError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsForbidden,
		message:   "Forbidden",
	}
}

// BeBadRequestError checks if error is BadRequest.
func BeBadRequestError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsBadRequest,
		message:   "BadRequest",
	}
}
