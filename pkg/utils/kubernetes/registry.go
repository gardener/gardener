// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils"
)

// ResolveRegistryCABundle resolves a registry CA bundle to its PEM-encoded certificate content.
// If inline is non-nil, it is returned directly. Otherwise the secret referenced by secretRef is
// read from the given namespace and the "bundle.crt" key is returned.
func ResolveRegistryCABundle(ctx context.Context, c client.Client, secretRef *corev1.SecretReference, inline *string) (string, error) {
	if inline != nil {
		return *inline, nil
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: secretRef.Namespace, Name: secretRef.Name}, secret); err != nil {
		return "", fmt.Errorf("failed to get registry CA bundle secret %s/%s: %w", secretRef.Namespace, secretRef.Name, err)
	}

	data, ok := secret.Data["bundle.crt"]
	if !ok {
		return "", fmt.Errorf("registry CA bundle secret %s/%s is missing key %q", secretRef.Namespace, secretRef.Name, "bundle.crt")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("registry CA bundle secret %s/%s has empty key %q", secretRef.Namespace, secretRef.Name, "bundle.crt")
	}

	cert, err := utils.DecodeCertificate(data)
	if err != nil {
		return "", fmt.Errorf("registry CA bundle secret %s/%s has invalid certificate in key %q: %w", secretRef.Namespace, secretRef.Name, "bundle.crt", err)
	}
	if time.Now().Add(7 * 24 * time.Hour).After(cert.NotAfter) {
		return "", fmt.Errorf("registry CA bundle secret %s/%s has a certificate in key %q that is either expired or expires within 7 days", secretRef.Namespace, secretRef.Name, "bundle.crt")
	}

	return string(data), nil
}
