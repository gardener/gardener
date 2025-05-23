// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/api"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	securityvalidation "github.com/gardener/gardener/pkg/apis/security/validation"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

// TokenRequestREST implements a RESTStorage for a token request.
type TokenRequestREST struct {
	coreInformerFactory gardencoreinformers.SharedInformerFactory

	workloadIdentityGetter getter
	tokenIssuer            workloadidentity.TokenIssuer
}

type getter interface {
	Get(context.Context, string, *metav1.GetOptions) (runtime.Object, error)
}

var (
	_   = rest.NamedCreater(&TokenRequestREST{})
	_   = rest.GroupVersionKindProvider(&TokenRequestREST{})
	gvk = securityv1alpha1.SchemeGroupVersion.WithKind("TokenRequest")
)

// New returns an instance of the object.
func (r *TokenRequestREST) New() runtime.Object {
	return &securityv1alpha1.TokenRequest{}
}

// Destroy cleans up its resources on shutdown.
func (r *TokenRequestREST) Destroy() {
	// Given that underlying store is shared with REST, we don't destroy it here explicitly.
}

// Create returns a TokenRequest with workload identity token based on
// - spec of the WorkloadIdentity
// - spec of the token request
// - referenced context object
// - gardener installation
func (r *TokenRequestREST) Create(ctx context.Context, name string, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	if r.tokenIssuer == nil {
		return nil, errors.New("TokenIssuer is not set, WorkloadIdentity tokens cannot be issued")
	}

	user, ok := genericapirequest.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("no user in context")
	}

	if createValidation != nil {
		if err := createValidation(ctx, obj.DeepCopyObject()); err != nil {
			return nil, err
		}
	}

	tokenRequest := &securityapi.TokenRequest{}
	if err := api.Scheme.Convert(obj, tokenRequest, nil); err != nil {
		return nil, fmt.Errorf("failed converting %T to %T: %w", obj, tokenRequest, err)
	}

	if len(tokenRequest.Name) != 0 && tokenRequest.Name != name {
		return nil, apierrors.NewInvalid(
			tokenRequest.GroupVersionKind().GroupKind(),
			tokenRequest.Name,
			field.ErrorList{
				field.Invalid(
					field.NewPath("metadata", "name"),
					tokenRequest.Name,
					"TokenRequest name does not match WorkloadIdentity name: "+name,
				),
			},
		)
	}

	namespace, ok := genericapirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("must specify namespace")
	}

	if len(tokenRequest.Namespace) != 0 && tokenRequest.Namespace != namespace {
		return nil, apierrors.NewInvalid(
			tokenRequest.GroupVersionKind().GroupKind(),
			tokenRequest.Namespace,
			field.ErrorList{
				field.Invalid(
					field.NewPath("metadata", "namespace"),
					tokenRequest.Namespace,
					"TokenRequest namespace does not match WorkloadIdentity namespace: "+namespace,
				),
			},
		)
	}

	workloadIdentityObj, err := r.workloadIdentityGetter.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	workloadIdentity, ok := workloadIdentityObj.(*securityapi.WorkloadIdentity)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("cannot convert to *security.WorkloadIdentity object - got type %T", workloadIdentityObj))
	}

	if len(tokenRequest.Name) == 0 {
		tokenRequest.Name = workloadIdentity.Name
	}

	if len(tokenRequest.Namespace) == 0 {
		tokenRequest.Namespace = workloadIdentity.Namespace
	}

	tokenRequest.CreationTimestamp = metav1.NewTime(time.Now())
	tokenRequest.ManagedFields = nil
	tokenRequest.Status = securityapi.TokenRequestStatus{}

	if errs := securityvalidation.ValidateTokenRequest(tokenRequest); len(errs) != 0 {
		return nil, apierrors.NewInvalid(gvk.GroupKind(), "", errs)
	}

	token, exp, err := r.issueToken(user, tokenRequest, workloadIdentity)
	if err != nil {
		return nil, err
	}

	tokenRequest.Status = securityapi.TokenRequestStatus{
		Token:               token,
		ExpirationTimestamp: metav1.Time{Time: *exp},
	}

	var out = &securityv1alpha1.TokenRequest{}
	if err = api.Scheme.Convert(tokenRequest, out, nil); err != nil {
		return nil, fmt.Errorf("failed converting %T to %T: %w", tokenRequest, out, err)
	}

	return out, nil
}

// GroupVersionKind returns the GVK for the workload identity request type.
func (r *TokenRequestREST) GroupVersionKind(schema.GroupVersion) schema.GroupVersionKind {
	return gvk
}

// NewTokenRequestREST returns a new TokenRequestREST for workload identity token.
func NewTokenRequestREST(
	storage getter,
	tokenIssuer workloadidentity.TokenIssuer,
	coreInformerFactory gardencoreinformers.SharedInformerFactory,
) *TokenRequestREST {
	return &TokenRequestREST{
		workloadIdentityGetter: storage,

		tokenIssuer:         tokenIssuer,
		coreInformerFactory: coreInformerFactory,
	}
}
