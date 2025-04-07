// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Builder is an object that builds Garden objects.
type Builder struct {
	projectFunc        func(context.Context) (*gardencorev1beta1.Project, error)
	internalDomainFunc func() (*gardenerutils.Domain, error)
	defaultDomainsFunc func() ([]*gardenerutils.Domain, error)
}

// Garden is an object containing Garden cluster specific data.
type Garden struct {
	Project        *gardencorev1beta1.Project
	DefaultDomains []*gardenerutils.Domain
	InternalDomain *gardenerutils.Domain
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

// WithInternalDomain sets the internalDomainFunc attribute at the Builder.
func (b *Builder) WithInternalDomain(internalDomain *gardenerutils.Domain) *Builder {
	b.internalDomainFunc = func() (*gardenerutils.Domain, error) { return internalDomain, nil }
	return b
}

// WithInternalDomainFromSecrets sets the internalDomainFunc attribute at the Builder based on the given secrets map.
func (b *Builder) WithInternalDomainFromSecrets(secrets map[string]*corev1.Secret) *Builder {
	b.internalDomainFunc = func() (*gardenerutils.Domain, error) { return gardenerutils.GetInternalDomain(secrets) }
	return b
}

// WithDefaultDomains sets the defaultDomainsFunc attribute at the Builder.
func (b *Builder) WithDefaultDomains(defaultDomains []*gardenerutils.Domain) *Builder {
	b.defaultDomainsFunc = func() ([]*gardenerutils.Domain, error) { return defaultDomains, nil }
	return b
}

// WithDefaultDomainsFromSecrets sets the defaultDomainsFunc attribute at the Builder based on the given secrets map.
func (b *Builder) WithDefaultDomainsFromSecrets(secrets map[string]*corev1.Secret) *Builder {
	b.defaultDomainsFunc = func() ([]*gardenerutils.Domain, error) { return gardenerutils.GetDefaultDomains(secrets) }
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

	if b.internalDomainFunc != nil {
		internalDomain, err := b.internalDomainFunc()
		if err != nil {
			return nil, err
		}
		garden.InternalDomain = internalDomain
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
