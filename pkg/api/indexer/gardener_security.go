// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package indexer

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

// CredentialsBindingCredentialsRefNameIndexerFunc extracts the .spec.credentialsRef.name field of a CredentialsBinding.
func CredentialsBindingCredentialsRefNameIndexerFunc(obj client.Object) []string {
	credentialsBinding, ok := obj.(*securityv1alpha1.CredentialsBinding)
	if !ok {
		return []string{""}
	}
	return []string{credentialsBinding.CredentialsRef.Name}
}

// CredentialsBindingCredentialsRefNamespaceIndexerFunc extracts the .spec.credentialsRef.namespace field of a CredentialsBinding.
func CredentialsBindingCredentialsRefNamespaceIndexerFunc(obj client.Object) []string {
	credentialsBinding, ok := obj.(*securityv1alpha1.CredentialsBinding)
	if !ok {
		return []string{""}
	}
	return []string{credentialsBinding.CredentialsRef.Namespace}
}

// CredentialsBindingCredentialsRefKindIndexerFunc extracts the .spec.credentialsRef.kind field of a CredentialsBinding.
func CredentialsBindingCredentialsRefKindIndexerFunc(obj client.Object) []string {
	credentialsBinding, ok := obj.(*securityv1alpha1.CredentialsBinding)
	if !ok {
		return []string{""}
	}
	return []string{credentialsBinding.CredentialsRef.Kind}
}

// AddCredentialsBindingCredentialsRefName adds an index for security.CredentialsBindingCredentialsRefName to the given indexer.
func AddCredentialsBindingCredentialsRefName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &securityv1alpha1.CredentialsBinding{}, security.CredentialsBindingCredentialsRefName, CredentialsBindingCredentialsRefNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to CredentialsBinding Informer: %w", security.CredentialsBindingCredentialsRefName, err)
	}
	return nil
}

// AddCredentialsBindingCredentialsRefNamespace adds an index for security.CredentialsBindingCredentialsRefNamespace to the given indexer.
func AddCredentialsBindingCredentialsRefNamespace(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &securityv1alpha1.CredentialsBinding{}, security.CredentialsBindingCredentialsRefNamespace, CredentialsBindingCredentialsRefNamespaceIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to CredentialsBinding Informer: %w", security.CredentialsBindingCredentialsRefNamespace, err)
	}
	return nil
}

// AddCredentialsBindingCredentialsRefKind adds an index for security.CredentialsBindingCredentialsRefKind to the given indexer.
func AddCredentialsBindingCredentialsRefKind(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &securityv1alpha1.CredentialsBinding{}, security.CredentialsBindingCredentialsRefKind, CredentialsBindingCredentialsRefKindIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to CredentialsBinding Informer: %w", security.CredentialsBindingCredentialsRefKind, err)
	}
	return nil
}
