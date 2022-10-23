// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"context"
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/version"
)

const (
	// SeedNamespaceNamePrefix is the prefix used for seed namespaces.
	SeedNamespaceNamePrefix = "seed-"
)

// ComputeGardenNamespace returns the name of the namespace belonging to the given seed in the Garden cluster.
func ComputeGardenNamespace(seedName string) string {
	return SeedNamespaceNamePrefix + seedName
}

// ComputeSeedName computes the name of the seed out of the seed namespace in the Garden cluster.
func ComputeSeedName(seedNamespaceName string) string {
	seedName := strings.TrimPrefix(seedNamespaceName, SeedNamespaceNamePrefix)
	if seedName == seedNamespaceName {
		return ""
	}
	return seedName
}

var (
	seedClientRequiredOrganization = []string{v1beta1constants.SeedsGroup}
	seedClientRequiredKeyUsages    = []certificatesv1.KeyUsage{
		certificatesv1.UsageKeyEncipherment,
		certificatesv1.UsageDigitalSignature,
		certificatesv1.UsageClientAuth,
	}
)

// IsSeedClientCert returns true when the given CSR and usages match the requirements for a client certificate for a
// seed. If false is returned, a reason will be returned explaining which requirement was not met.
func IsSeedClientCert(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage) (bool, string) {
	if !reflect.DeepEqual(seedClientRequiredOrganization, x509cr.Subject.Organization) {
		return false, fmt.Sprintf("subject's organization is not set to %v", seedClientRequiredOrganization)
	}

	if (len(x509cr.DNSNames) > 0) || (len(x509cr.EmailAddresses) > 0) || (len(x509cr.IPAddresses) > 0) {
		return false, "DNSNames, EmailAddresses and IPAddresses fields must be empty"
	}

	if !hasExactUsages(usages, seedClientRequiredKeyUsages) {
		return false, fmt.Sprintf("key usages are not set to %v", seedClientRequiredKeyUsages)
	}

	if !strings.HasPrefix(x509cr.Subject.CommonName, v1beta1constants.SeedUserNamePrefix) {
		return false, fmt.Sprintf("CommonName does not start with %q", v1beta1constants.SeedUserNamePrefix)
	}

	return true, ""
}

func hasExactUsages(usages, requiredUsages []certificatesv1.KeyUsage) bool {
	if len(requiredUsages) != len(usages) {
		return false
	}

	usageMap := map[certificatesv1.KeyUsage]struct{}{}
	for _, u := range usages {
		usageMap[u] = struct{}{}
	}

	for _, u := range requiredUsages {
		if _, ok := usageMap[u]; !ok {
			return false
		}
	}

	return true
}

// ComputeNginxIngressClassForSeed returns the IngressClass for the Nginx Ingress controller.
func ComputeNginxIngressClassForSeed(seed *gardencorev1beta1.Seed, kubernetesVersion *string) (string, error) {
	if kubernetesVersion == nil {
		return "", fmt.Errorf("kubernetes version is missing for seed %q", seed.Name)
	}

	// We need to use `versionutils.CompareVersions` because this function normalizes the seed version first.
	// This is especially necessary if the seed cluster is a non Gardener managed cluster and thus might have some
	// custom version suffix.
	greaterEqual122, err := version.CompareVersions(*kubernetesVersion, ">=", "1.22")
	if err != nil {
		return "", err
	}

	if managed := helper.SeedWantsManagedIngress(seed); managed {
		if greaterEqual122 {
			return v1beta1constants.SeedNginxIngressClass122, nil
		} else {
			return v1beta1constants.SeedNginxIngressClass, nil
		}
	}

	return v1beta1constants.NginxIngressClass, nil
}

// GetWildcardCertificate gets the wildcard certificate for the seed's ingress domain.
// Nil is returned if no wildcard certificate is configured.
func GetWildcardCertificate(ctx context.Context, c client.Client) (*corev1.Secret, error) {
	wildcardCerts := &corev1.SecretList{}
	if err := c.List(
		ctx,
		wildcardCerts,
		client.InNamespace(v1beta1constants.GardenNamespace),
		client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlaneWildcardCert},
	); err != nil {
		return nil, err
	}

	if len(wildcardCerts.Items) > 1 {
		return nil, fmt.Errorf("misconfigured seed cluster: not possible to provide more than one secret with annotation %s", v1beta1constants.GardenRoleControlPlaneWildcardCert)
	}

	if len(wildcardCerts.Items) == 1 {
		return &wildcardCerts.Items[0], nil
	}
	return nil, nil
}
