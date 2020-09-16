// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// Builder is an object that builds Garden objects.
type Builder struct {
	projectFunc        func() (*gardencorev1beta1.Project, error)
	internalDomainFunc func() (*Domain, error)
	defaultDomainsFunc func() ([]*Domain, error)
}

// Garden is an object containing Garden cluster specific data.
type Garden struct {
	Project        *gardencorev1beta1.Project
	DefaultDomains []*Domain
	InternalDomain *Domain
}

// Domain contains information about a domain configured in the garden cluster.
type Domain struct {
	Domain         string
	Provider       string
	SecretData     map[string][]byte
	IncludeDomains []string
	ExcludeDomains []string
	IncludeZones   []string
	ExcludeZones   []string
}
