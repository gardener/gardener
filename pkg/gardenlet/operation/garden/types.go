// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package garden

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Builder is an object that builds Garden objects.
type Builder struct {
	projectFunc                       func(context.Context) (*gardencorev1beta1.Project, error)
	internalDomainFunc                func() (*gardenerutils.Domain, error)
	defaultDomainsFunc                func() ([]*gardenerutils.Domain, error)
	shootServiceAccountIssuerHostname func() (*string, error)
}

// Garden is an object containing Garden cluster specific data.
type Garden struct {
	Project                           *gardencorev1beta1.Project
	DefaultDomains                    []*gardenerutils.Domain
	InternalDomain                    *gardenerutils.Domain
	ShootServiceAccountIssuerHostname *string
}

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		projectFunc: func(context.Context) (*gardencorev1beta1.Project, error) {
			return nil, fmt.Errorf("project is required but not set")
		},
		internalDomainFunc: func() (*gardenerutils.Domain, error) {
			return nil, fmt.Errorf("internal domain is required but not set")
		},
		shootServiceAccountIssuerHostname: func() (*string, error) {
			return nil, fmt.Errorf("shoot service account issuer hostname func is required but not set")
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

// WithProjectFrom sets the projectFunc attribute after fetching it from the given reader.
func (b *Builder) WithShootServiceAccountIssuerHostname(secrets map[string]*corev1.Secret) *Builder {
	b.shootServiceAccountIssuerHostname = func() (*string, error) {
		if s, ok := secrets[v1beta1constants.GardenRoleShootServiceAccountIssuer]; ok {
			if host, ok := s.Data["hostname"]; ok {
				hostname := string(host)
				return &hostname, nil
			}
			return nil, errors.New("shoot service account issuer secret is missing a hostname key")
		}
		return nil, nil
	}
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

	internalDomain, err := b.internalDomainFunc()
	if err != nil {
		return nil, err
	}
	garden.InternalDomain = internalDomain

	defaultDomains, err := b.defaultDomainsFunc()
	if err != nil {
		return nil, err
	}
	garden.DefaultDomains = defaultDomains

	shootServiceAccountIssuerHostname, err := b.shootServiceAccountIssuerHostname()
	if err != nil {
		return nil, err
	}
	garden.ShootServiceAccountIssuerHostname = shootServiceAccountIssuerHostname

	return garden, nil
}
