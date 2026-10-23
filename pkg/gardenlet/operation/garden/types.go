// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Builder is an object that builds Garden objects.
type Builder struct {
	projectFunc         func(context.Context) (*gardencorev1beta1.Project, error)
	internalDomainsFunc func() ([]*gardenerutils.Domain, error)
	defaultDomainsFunc  func() ([]*gardenerutils.Domain, error)
}

// Garden is an object containing Garden cluster specific data.
type Garden struct {
	Project         *gardencorev1beta1.Project
	DefaultDomains  []*gardenerutils.Domain
	InternalDomains []*gardenerutils.Domain
}

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		projectFunc: func(context.Context) (*gardencorev1beta1.Project, error) {
			return nil, fmt.Errorf("project is required but not set")
		},
	}
}

// WithProject sets the projectFunc attribute at the Builder.
func (b *Builder) WithProject(project *gardencorev1beta1.Project) *Builder {
	b.projectFunc = func(context.Context) (*gardencorev1beta1.Project, error) { return project, nil }
	return b
}

// WithProjectFrom sets the projectFunc attribute after fetching it from the given reader.
func (b *Builder) WithProjectFrom(reader client.Reader, namespace string) *Builder {
	b.projectFunc = func(ctx context.Context) (*gardencorev1beta1.Project, error) {
		project, _, err := gardenerutils.ProjectAndNamespaceFromReader(ctx, reader, namespace)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, fmt.Errorf("cannot find Project for namespace '%s'", namespace)
		}

		return project, err
	}
	return b
}

// WithInternalDomains sets the internalDomainsFunc attribute at the Builder.
func (b *Builder) WithInternalDomains(internalDomains []*gardenerutils.Domain) *Builder {
	b.internalDomainsFunc = func() ([]*gardenerutils.Domain, error) { return internalDomains, nil }
	return b
}

// WithDefaultDomains sets the defaultDomainsFunc attribute at the Builder.
func (b *Builder) WithDefaultDomains(defaultDomains []*gardenerutils.Domain) *Builder {
	b.defaultDomainsFunc = func() ([]*gardenerutils.Domain, error) { return defaultDomains, nil }
	return b
}

// Build initializes a new Garden object.
func (b *Builder) Build(ctx context.Context) (*Garden, error) {
	garden := &Garden{}

	project, err := b.projectFunc(ctx)
	if err != nil {
		return nil, err
	}
	garden.Project = project

	if b.internalDomainsFunc != nil {
		internalDomains, err := b.internalDomainsFunc()
		if err != nil {
			return nil, err
		}
		garden.InternalDomains = internalDomains
	}

	if b.defaultDomainsFunc != nil {
		defaultDomains, err := b.defaultDomainsFunc()
		if err != nil {
			return nil, err
		}
		garden.DefaultDomains = defaultDomains
	}

	return garden, nil
}

// SeedInternalDomains returns the internal domains of the given seed. Without the MutableShootDomains feature gate, it
// only returns spec.dns.internal.
func SeedInternalDomains(seed *gardencorev1beta1.Seed) []gardencorev1beta1.SeedDNSProviderConfig {
	if features.DefaultFeatureGate.Enabled(features.MutableShootDomains) {
		return seed.Spec.DNS.InternalDomains
	}
	if seed.Spec.DNS.Internal == nil {
		return nil
	}
	return []gardencorev1beta1.SeedDNSProviderConfig{*seed.Spec.DNS.Internal}
}
