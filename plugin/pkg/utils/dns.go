// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/operation/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
)

// GetDefaultDomainSecrets filters the secrets in the Garden namespace for default domain secrets.
func GetDefaultDomainSecrets(secretLister kubecorev1listers.SecretLister) ([]corev1.Secret, error) {
	defaultDomainSecrets := []corev1.Secret{}

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", common.GardenRole, common.GardenRoleDefaultDomain))
	if err != nil {
		return nil, err
	}
	secrets, err := secretLister.Secrets(common.GardenNamespace).List(selector)
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets {
		metadata := secret.ObjectMeta
		if !metav1.HasAnnotation(metadata, common.DNSHostedZoneID) || !metav1.HasAnnotation(metadata, common.DNSProvider) || !metav1.HasAnnotation(metadata, common.DNSDomain) {
			continue
		}
		defaultDomainSecrets = append(defaultDomainSecrets, *secret)
	}

	return defaultDomainSecrets, nil
}

// GetDefaultDomainSecret tries to find a matching secret for the default domain passed in the dns object.
func GetDefaultDomainSecret(secretLister kubecorev1listers.SecretLister, dns *garden.DNS) (*corev1.Secret, error) {
	secrets, err := GetDefaultDomainSecrets(secretLister)
	if err != nil {
		return nil, err
	}
	for _, secret := range secrets {
		if strings.HasSuffix(*(dns.Domain), secret.Annotations[common.DNSDomain]) && dns.Provider == garden.DNSProvider(secret.Annotations[common.DNSProvider]) {
			return &secret, nil
		}
	}
	return nil, nil
}

// GetInternalDomainSecret gets access information for the internal domain of the Garden cluster.
func GetInternalDomainSecret(secretLister kubecorev1listers.SecretLister) (*corev1.Secret, error) {
	selector, err := labels.Parse(fmt.Sprintf("%s=%s", common.GardenRole, common.GardenRoleInternalDomain))
	if err != nil {
		return nil, err
	}
	secrets, err := secretLister.List(selector)
	if err != nil {
		return nil, err
	}

	numberOfInternalDomainSecrets := len(secrets)
	if numberOfInternalDomainSecrets != 1 {
		return nil, fmt.Errorf("require exactly ONE internal domain secret, but found %d", numberOfInternalDomainSecrets)
	}

	return secrets[0], nil
}

// HasDomainIntersection checks if domainA is a suffix of domainB or domainB is a suffix of domainA.
func HasDomainIntersection(domainA, domainB string) bool {
	if domainA == domainB {
		return true
	}

	var short, long string
	if len(domainA) > len(domainB) {
		short = domainB
		long = domainA
	} else {
		short = domainA
		long = domainB
	}
	return strings.HasSuffix(long, short)
}
