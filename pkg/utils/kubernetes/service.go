// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// DNSNamesForService returns the possible DNS names for a service with the given name and namespace.
func DNSNamesForService(name, namespace string) []string {
	return []string{
		name,
		fmt.Sprintf("%s.%s", name, namespace),
		fmt.Sprintf("%s.%s.svc", name, namespace),
		fmt.Sprintf("%s.%s.svc.%s", name, namespace, v1beta1.DefaultDomain),
	}
}
