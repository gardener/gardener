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

package secrets

import (
	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Secrets represents a set of secrets that can be deployed and deleted.
type Secrets struct {
	CertificateSecretConfigs map[string]*CertificateSecretConfig
	SecretConfigsFunc        func(map[string]*Certificate, string) []ConfigInterface
}

// Deploy generates and deploys the secrets into the given namespace, taking into account existing secrets.
func (s *Secrets) Deploy(
	cs kubernetes.Interface,
	gcs gardenerkubernetes.Interface,
	namespace string,
) (map[string]*corev1.Secret, error) {

	// Get existing secrets in the namespace
	existingSecrets, err := getSecrets(cs, namespace)
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
	clusterSecrets, err := GenerateClusterSecrets(gcs, existingSecrets, secretConfigs, namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "could not generate cluster secrets in namespace '%s'", namespace)
	}

	return clusterSecrets, nil
}

// Delete deletes the secrets from the given namespace.
func (s *Secrets) Delete(cs kubernetes.Interface, namespace string) error {
	for _, sc := range s.SecretConfigsFunc(nil, namespace) {
		if err := deleteSecret(cs, namespace, sc.GetName()); err != nil {
			return err
		}
	}
	return nil
}

func getSecrets(cs kubernetes.Interface, namespace string) (map[string]*corev1.Secret, error) {
	secretList, err := cs.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
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

func deleteSecret(cs kubernetes.Interface, namespace, name string) error {
	err := cs.CoreV1().Secrets(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "could not delete secret '%s/%s'", namespace, name)
	}
	return nil
}
