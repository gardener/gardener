// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/api"
	securityapi "github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	securityvalidation "github.com/gardener/gardener/pkg/apis/security/validation"
)

// TokenRequestREST implements a RESTStorage for a token request.
type TokenRequestREST struct {
	// client kubernetes.Interface // Might be needed later to read the context object

	minDuration            int64
	maxDuration            int64
	workloadIdentityGetter getter
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
// - spec of the workload identity
// - spec of the token request
// - referenced context object
// - gardener installation
func (r *TokenRequestREST) Create(ctx context.Context, name string, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	if createValidation != nil {
		if err := createValidation(ctx, obj.DeepCopyObject()); err != nil {
			return nil, err
		}
	}

	tokenRequest := &securityapi.TokenRequest{}
	if err := api.Scheme.Convert(obj, tokenRequest, nil); err != nil {
		return nil, fmt.Errorf("failed converting %T to %T: %w", obj, tokenRequest, err)
	}
	if errs := securityvalidation.ValidateTokenRequest(tokenRequest); len(errs) != 0 {
		return nil, apierrors.NewInvalid(gvk.GroupKind(), "", errs)
	}

	// TODO(vpnachev): implement context specific features
	// if tokenRequest.Spec.ContextObject != nil {	}

	workloadIdentityObj, err := r.workloadIdentityGetter.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	workloadIdentity, ok := workloadIdentityObj.(*securityapi.WorkloadIdentity)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("cannot convert to *security.WorkloadIdentity object - got type %T", workloadIdentityObj))
	}

	var (
		aud      = workloadIdentity.Spec.Audiences
		sub      = workloadIdentity.Status.Sub
		duration = tokenRequest.Spec.DurationSeconds
	)

	if duration < r.minDuration {
		duration = r.minDuration
	} else if duration > r.maxDuration {
		duration = r.maxDuration
	}

	var (
		now = time.Now()
		exp = now.Add(time.Second * time.Duration(duration))
	)
	token, err := issueToken(aud, sub, now, now, exp)
	if err != nil {
		return nil, err
	}
	tokenRequest.Status.Token = token
	tokenRequest.Status.ExpirationTimeStamp = metav1.Time{Time: exp}

	return tokenRequest, nil
}

// issueToken generates JWT out of the provided configs.
func issueToken(aud []string, sub string, iat, nbf, exp time.Time) (string, error) {
	issuer := "https://issuer.gardener.cloud" // TODO(vpnachev): Make issuer configurable

	// TODO(vpnachev): Implement real JWT issuer.
	token := struct {
		Iss string    `json:"iss"`
		Sub string    `json:"sub"`
		Aud []string  `json:"aud"`
		Iat time.Time `json:"iat"`
		Nbf time.Time `json:"nbf"`
		Exp time.Time `json:"exp"`
	}{
		Iss: issuer,
		Sub: sub,
		Aud: aud,
		Iat: iat,
		Nbf: nbf,
		Exp: exp,
	}

	t, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return string(t), nil
}

// GroupVersionKind returns the GVK for the workload identity request type.
func (r *TokenRequestREST) GroupVersionKind(schema.GroupVersion) schema.GroupVersionKind {
	return gvk
}

// NewTokenRequestREST returns a new TokenRequestREST for workload identity token.
func NewTokenRequestREST(
	storage getter,
	minDuration,
	maxDuration time.Duration,
) *TokenRequestREST {
	return &TokenRequestREST{
		workloadIdentityGetter: storage,

		minDuration: int64(minDuration.Seconds()),
		maxDuration: int64(maxDuration.Seconds()),
	}
}
