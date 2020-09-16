// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"

	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Interface represents a set of secrets that can be deployed and deleted.
type Interface interface {
	// Deploy generates and deploys the secrets into the given namespace, taking into account existing secrets.
	Deploy(context.Context, kubernetes.Interface, gardenerkubernetes.Interface, string) (map[string]*corev1.Secret, error)
	// Delete deletes the secrets from the given namespace.
	Delete(kubernetes.Interface, string) error
}

// Secrets represents a set of secrets that can be deployed and deleted.
type Secrets struct {
	CertificateSecretConfigs map[string]*CertificateSecretConfig
	SecretConfigsFunc        func(map[string]*Certificate, string) []ConfigInterface
}

// Deploy generates and deploys the secrets into the given namespace, taking into account existing secrets.
func (s *Secrets) Deploy(
	ctx context.Context,
	cs kubernetes.Interface,
	gcs gardenerkubernetes.Interface,
	namespace string,
) (map[string]*corev1.Secret, error) {
	// Get existing secrets in the namespace
	existingSecrets, err := getSecrets(ctx, cs, namespace)
	if err != nil {
		return nil, err
	}

	// Generate CAs
	_, cas, err := GenerateCertificateAuthorities(gcs, existingSecrets, s.CertificateSecretConfigs, namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "could not generate CA secrets in namespace '%s'", namespace)
	}

	// Generate cluster secrets
	secretConfigs := s.SecretConfigsFunc(cas, namespace)
	clusterSecrets, err := GenerateClusterSecrets(ctx, gcs, existingSecrets, secretConfigs, namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "could not generate cluster secrets in namespace '%s'", namespace)
	}

	return clusterSecrets, nil
}

// Delete deletes the secrets from the given namespace.
func (s *Secrets) Delete(ctx context.Context, cs kubernetes.Interface, namespace string) error {
	for _, sc := range s.SecretConfigsFunc(nil, namespace) {
		if err := deleteSecret(ctx, cs, namespace, sc.GetName()); err != nil {
			return err
		}
	}
	return nil
}

func getSecrets(ctx context.Context, cs kubernetes.Interface, namespace string) (map[string]*corev1.Secret, error) {
	secretList, err := cs.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "could not list secrets in namespace '%s'", namespace)
	}
	result := make(map[string]*corev1.Secret, len(secretList.Items))
	for _, secret := range secretList.Items {
		func(secret corev1.Secret) {
			result[secret.Name] = &secret
		}(secret)
	}
	return result, nil
}

func deleteSecret(ctx context.Context, cs kubernetes.Interface, namespace, name string) error {
	return cs.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
