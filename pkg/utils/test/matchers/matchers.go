// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"context"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testruntime "github.com/gardener/gardener/pkg/utils/test/runtime"
)

func init() {
	// if CharactersAroundMismatchToInclude is too small, then format.MessageWithDiff will be unable to output our
	// mismatch message
	// set the variable in init func, otherwise the race detector will complain when matchers are used concurrently in
	// multiple goroutines
	if format.CharactersAroundMismatchToInclude < 50 {
		format.CharactersAroundMismatchToInclude = 50
	}
}

// DeepEqual returns a Gomega matcher which checks whether the expected object is deeply equal with the object it is
// being compared against.
func DeepEqual(expected interface{}) types.GomegaMatcher {
	return newDeepEqualMatcher(expected)
}

// DeepDerivativeEqual is similar to DeepEqual except that unset fields in actual are
// ignored (not compared). This allows us to focus on the fields that matter to
// the semantic comparison.
func DeepDerivativeEqual(expected interface{}) types.GomegaMatcher {
	return newDeepDerivativeMatcher(expected)
}

// BeNotFoundError checks if error is NotFound.
func BeNotFoundError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsNotFound,
		message:   "NotFound",
	}
}

// BeNotRegisteredError checks if error is NotRegistered.
func BeNotRegisteredError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: runtime.IsNotRegisteredError,
		message:   "NotRegistered",
	}
}

// BeAlreadyExistsError checks if error is AlreadyExists.
func BeAlreadyExistsError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsAlreadyExists,
		message:   "AlreadyExists",
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

// BeNoMatchError checks if error is a NoMatchError.
func BeNoMatchError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: meta.IsNoMatchError,
		message:   "NoMatch",
	}
}

// BeMissingKindError checks if error is a MissingKindError.
func BeMissingKindError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: runtime.IsMissingKind,
		message:   "Object 'Kind' is missing",
	}
}

// BeInternalServerError checks if error is a InternalServerError.
func BeInternalServerError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsInternalError,
		message:   "",
	}
}

// BeInvalidError checks if error is an InvalidError.
func BeInvalidError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsInvalid,
		message:   "Invalid",
	}
}

// ShareSameReferenceAs checks if objects shares the same underlying reference as the passed object.
// This can be used to check if maps or slices have the same underlying data store.
// Only objects that work for 'reflect.ValueOf(x).Pointer' can be compared.
func ShareSameReferenceAs(expected interface{}) types.GomegaMatcher {
	return &referenceMatcher{
		expected: expected,
	}
}

// NewManagedResourceContainsObjectsMatcher returns a function for a matcher that checks
// if the given objects are handled by the given managed resource.
// It is expected that the data keys of referenced secret(s) follow the semantics of `managedresources.Registry`.
func NewManagedResourceContainsObjectsMatcher(cl client.Client) func(...client.Object) types.GomegaMatcher {
	return func(objs ...client.Object) types.GomegaMatcher {
		expectedObjToSerialized := make(map[client.Object]string)
		for _, o := range objs {
			obj := o
			expectedObjToSerialized[obj] = testruntime.Serialize(obj, cl.Scheme())
		}

		return &managedResourceObjectsMatcher{
			ctx:                     context.Background(),
			cl:                      cl,
			expectedObjToSerialized: expectedObjToSerialized,
		}
	}
}

// NewManagedResourceConsistOfObjectsMatcher returns a function for a matcher that checks
// if the exact list of given objects are handled by the given managed resource.
// Any extra objects found through the ManagedResource let the matcher fail.
// It is expected that the data keys of referenced secret(s) follow the semantics of `managedresources.Registry`.
func NewManagedResourceConsistOfObjectsMatcher(cl client.Client) func(...client.Object) types.GomegaMatcher {
	return func(objs ...client.Object) types.GomegaMatcher {
		expectedObjToSerialized := make(map[client.Object]string)
		for _, o := range objs {
			obj := o
			expectedObjToSerialized[obj] = testruntime.Serialize(obj, cl.Scheme())
		}

		return &managedResourceObjectsMatcher{
			ctx:                     context.Background(),
			cl:                      cl,
			expectedObjToSerialized: expectedObjToSerialized,
			extraObjectsCheck:       true,
		}
	}
}
