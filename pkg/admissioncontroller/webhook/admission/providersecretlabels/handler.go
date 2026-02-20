// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package providersecretlabels

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

// Handler syncs the provider labels on Secrets referenced in SecretBindings, or on Secrets and InternalSecrets
// referenced in CredentialsBindings.
type Handler struct {
	Logger logr.Logger
	Client client.Client

	secretHandler         admission.Handler
	internalSecretHandler admission.Handler
}

// Handle syncs the provider labels.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	requestResource := schema.GroupResource{Group: req.Resource.Group, Resource: req.Resource.Resource}
	switch requestResource {
	case corev1.Resource("secrets"):
		return h.secretHandler.Handle(ctx, req)
	case gardencorev1beta1.Resource("internalsecrets"):
		return h.internalSecretHandler.Handle(ctx, req)
	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected resource %s", req.Resource.String()))
	}
}

// SecretHandler syncs the provider labels on Secrets referenced in SecretBindings or CredentialsBindings.
type SecretHandler struct {
	*Handler
}

// Default syncs the provider labels for Secrets.
func (h *SecretHandler) Default(ctx context.Context, obj runtime.Object) error {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("expected secret but got %T", obj)
	}

	typesFromSecretBindings, err := h.fetchProviderTypesFromSecretBindings(ctx, secret.Name, secret.Namespace)
	if err != nil {
		return fmt.Errorf("failed fetching provider types from SecretBindings: %w", err)
	}

	typesFromCredentialsBindings, err := h.fetchProviderTypesFromCredentialsBindings(ctx, corev1.SchemeGroupVersion.String(), "Secret", secret.Name, secret.Namespace)
	if err != nil {
		return fmt.Errorf("failed fetching provider types from CredentialsBindings: %w", err)
	}

	if allTypes := typesFromSecretBindings.Union(typesFromCredentialsBindings); allTypes.Len() > 0 {
		maintainLabels(secret, allTypes.UnsortedList()...)
	}

	return nil
}

// InternalSecretHandler syncs the provider labels on InternalSecrets referenced in CredentialsBindings.
type InternalSecretHandler struct {
	*Handler
}

// Default syncs the provider labels for InternalSecrets.
func (h *InternalSecretHandler) Default(ctx context.Context, obj runtime.Object) error {
	internalSecret, ok := obj.(*gardencorev1beta1.InternalSecret)
	if !ok {
		return fmt.Errorf("expected InternalSecret but got %T", obj)
	}

	typesFromCredentialsBindings, err := h.fetchProviderTypesFromCredentialsBindings(ctx, gardencorev1beta1.SchemeGroupVersion.String(), "InternalSecret", internalSecret.Name, internalSecret.Namespace)
	if err != nil {
		return fmt.Errorf("failed fetching provider types from CredentialsBindings: %w", err)
	}

	if typesFromCredentialsBindings.Len() > 0 {
		maintainLabels(internalSecret, typesFromCredentialsBindings.UnsortedList()...)
	}

	return nil
}

func (h *Handler) fetchProviderTypesFromSecretBindings(ctx context.Context, name, namespace string) (sets.Set[string], error) {
	secretBindingList := &gardencorev1beta1.SecretBindingList{}
	if err := h.Client.List(ctx, secretBindingList); err != nil {
		return nil, fmt.Errorf("failed to list SecretBindings: %w", err)
	}

	providerTypes := sets.New[string]()
	for _, secretBinding := range secretBindingList.Items {
		if secretBinding.SecretRef.Name == name &&
			secretBinding.SecretRef.Namespace == namespace {
			providerTypes.Insert(v1beta1helper.GetSecretBindingTypes(&secretBinding)...)
		}
	}
	return providerTypes, nil
}

func (h *Handler) fetchProviderTypesFromCredentialsBindings(ctx context.Context, apiVersion, kind, name, namespace string) (sets.Set[string], error) {
	credentialsBindingList := &securityv1alpha1.CredentialsBindingList{}
	if err := h.Client.List(ctx, credentialsBindingList); err != nil {
		return nil, fmt.Errorf("failed to list CredentialsBindings: %w", err)
	}

	providerTypes := sets.New[string]()
	for _, credentialsBinding := range credentialsBindingList.Items {
		if credentialsBinding.CredentialsRef.APIVersion == apiVersion &&
			credentialsBinding.CredentialsRef.Kind == kind &&
			credentialsBinding.CredentialsRef.Name == name &&
			credentialsBinding.CredentialsRef.Namespace == namespace {
			providerTypes.Insert(credentialsBinding.Provider.Type)
		}
	}
	return providerTypes, nil
}

func maintainLabels(obj client.Object, providerTypes ...string) {
	labels := obj.GetLabels()

	for k := range labels {
		if strings.HasPrefix(k, v1beta1constants.LabelShootProviderPrefix) {
			delete(labels, k)
		}
	}

	if len(providerTypes) > 0 && labels == nil {
		labels = make(map[string]string)
	}

	for _, providerType := range providerTypes {
		labels[v1beta1constants.LabelShootProviderPrefix+providerType] = "true"
	}

	obj.SetLabels(labels)
}
