// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/security"
	"github.com/gardener/gardener/pkg/apis/security/validation"
)

type workloadIdentityStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator

	tokenIssuerURL *string
}

// NewStrategy creates new storage strategy for WorkloadIdentity.
func NewStrategy(tokenIssuerURL *string) workloadIdentityStrategy {
	return workloadIdentityStrategy{api.Scheme, names.SimpleNameGenerator, tokenIssuerURL}
}

func (workloadIdentityStrategy) NamespaceScoped() bool {
	return true
}

func (s workloadIdentityStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	// During creation, the resource name (when generateName is used) and UID are set after all mutating plugins,
	// also after defaulting. Because of this, it is not possible to set the `status.sub` field
	// neither with the static default, nor with admission plugin (built-in or dynamic webhook).
	// ref: https://github.com/kubernetes/kubernetes/issues/46107#issuecomment-537601110

	wi := obj.(*security.WorkloadIdentity)
	if string(wi.GetUID()) == "" {
		wi.SetUID(uuid.NewUUID())
	}

	if wi.GetName() == "" {
		wi.SetName(s.GenerateName(wi.GetGenerateName()))
	}

	p, d := validation.GetSubClaimPrefixAndDelimiterFunc()
	if wi.Status.Sub == "" {
		wi.Status.Sub = strings.Join([]string{p, wi.GetNamespace(), wi.GetName(), string(wi.GetUID())}, d)
	}
	wi.Status.Issuer = s.tokenIssuerURL
}

func (s workloadIdentityStrategy) PrepareForUpdate(_ context.Context, obj, _ runtime.Object) {
	wi := obj.(*security.WorkloadIdentity)
	wi.Status.Issuer = s.tokenIssuerURL
}

func (workloadIdentityStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	workloadidentity := obj.(*security.WorkloadIdentity)
	return validation.ValidateWorkloadIdentity(workloadidentity)
}

func (workloadIdentityStrategy) Canonicalize(_ runtime.Object) {
}

func (workloadIdentityStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (workloadIdentityStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newWorkloadIdentity := newObj.(*security.WorkloadIdentity)
	oldWorkloadIdentity := oldObj.(*security.WorkloadIdentity)
	return validation.ValidateWorkloadIdentityUpdate(newWorkloadIdentity, oldWorkloadIdentity)
}

func (workloadIdentityStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (workloadIdentityStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (workloadIdentityStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}
