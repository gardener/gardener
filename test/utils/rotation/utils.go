// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"
	"sort"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretConfigNamesToSecrets is a map for secret config names to a list of corev1.Secret objects.
type SecretConfigNamesToSecrets map[string][]corev1.Secret

// GroupByName groups all secrets by name.
func GroupByName(allSecrets []corev1.Secret) SecretConfigNamesToSecrets {
	grouped := make(SecretConfigNamesToSecrets)
	for _, secret := range allSecrets {
		grouped[secret.Labels["name"]] = append(grouped[secret.Labels["name"]], secret)
	}

	for _, secrets := range grouped {
		sort.Sort(AgeSorter(secrets))
	}
	return grouped
}

// AgeSorter implements sort.Interface for a slice of secrets for sorting by age.
type AgeSorter []corev1.Secret

func (x AgeSorter) Len() int           { return len(x) }
func (x AgeSorter) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x AgeSorter) Less(i, j int) bool { return x[i].CreationTimestamp.Before(&x[j].CreationTimestamp) }

// VerifyEncryptedData creates and reads encrypted resources in the target cluster to verify encryption is working.
func VerifyEncryptedData(ctx context.Context, c client.Client, resources []EncryptedResource) {
	for _, resource := range resources {
		obj := resource.NewObject()
		Eventually(func(g Gomega) {
			g.Expect(c.Create(ctx, obj)).To(Succeed())
		}).Should(Succeed(), "creating resource should succeed for "+client.ObjectKeyFromObject(obj).String())

		Eventually(func(g Gomega) {
			g.Expect(c.List(ctx, resource.NewEmptyList())).To(Succeed())
		}).Should(Succeed(), "reading all encrypted resources should succeed")
	}
}
