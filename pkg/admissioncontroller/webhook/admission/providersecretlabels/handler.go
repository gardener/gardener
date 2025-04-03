// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package providersecretlabels

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

// Handler syncs the provider labels on Secrets referenced in SecretBindings or CredentialsBindings.
type Handler struct {
	Logger logr.Logger
	Client client.Client
}

// Default syncs the provider labels.
func (h *Handler) Default(ctx context.Context, obj runtime.Object) error {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("expected secret but got %T", obj)
	}

	typesFromSecretBindings, err := h.fetchProviderTypesFromSecretBindings(ctx, secret)
	if err != nil {
		return fmt.Errorf("failed fetching provider types from SecretBindings: %w", err)
	}

	typesFromCredentialsBindings, err := h.fetchProviderTypesFromCredentialsBindings(ctx, secret)
	if err != nil {
		return fmt.Errorf("failed fetching provider types from CredentialsBindings: %w", err)
	}

	if len(typesFromSecretBindings)+len(typesFromCredentialsBindings) > 0 {
		maintainLabels(secret, sets.New(typesFromSecretBindings...).Insert(typesFromCredentialsBindings...).UnsortedList()...)
	}

	return nil
}

func (h *Handler) fetchProviderTypesFromSecretBindings(ctx context.Context, secret *corev1.Secret) ([]string, error) {
	secretBindingList := &gardencorev1beta1.SecretBindingList{}
	if err := h.Client.List(ctx, secretBindingList); err != nil {
		return nil, fmt.Errorf("failed to list SecretBindings: %w", err)
	}

	var providerTypes []string
	for _, secretBinding := range secretBindingList.Items {
		if secretBinding.SecretRef.Name == secret.Name &&
			secretBinding.SecretRef.Namespace == secret.Namespace {
			providerTypes = append(providerTypes, v1beta1helper.GetSecretBindingTypes(&secretBinding)...)
		}
	}
	return providerTypes, nil
}

func (h *Handler) fetchProviderTypesFromCredentialsBindings(ctx context.Context, secret *corev1.Secret) ([]string, error) {
	credentialsBindingList := &securityv1alpha1.CredentialsBindingList{}
	if err := h.Client.List(ctx, credentialsBindingList); err != nil {
		return nil, fmt.Errorf("failed to list CredentialsBindings: %w", err)
	}

	var providerTypes []string
	for _, credentialsBinding := range credentialsBindingList.Items {
		if credentialsBinding.CredentialsRef.APIVersion == corev1.SchemeGroupVersion.String() &&
			credentialsBinding.CredentialsRef.Kind == "Secret" &&
			credentialsBinding.CredentialsRef.Name == secret.Name &&
			credentialsBinding.CredentialsRef.Namespace == secret.Namespace {
			providerTypes = append(providerTypes, credentialsBinding.Provider.Type)
		}
	}
	return providerTypes, nil
}

func maintainLabels(secret *corev1.Secret, providerTypes ...string) {
	for k := range secret.Labels {
		if strings.HasPrefix(k, v1beta1constants.LabelShootProviderPrefix) {
			delete(secret.Labels, k)
		}
	}

	for _, providerType := range providerTypes {
		metav1.SetMetaDataLabel(&secret.ObjectMeta, v1beta1constants.LabelShootProviderPrefix+providerType, "true")
	}
}
