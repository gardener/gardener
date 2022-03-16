// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package manager

import (
	"context"
	"fmt"
	"strings"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (m *manager) Generate(ctx context.Context, config secretutils.ConfigInterface, opts ...GenerateOption) (*corev1.Secret, error) {
	options := &GenerateOptions{}
	if err := options.ApplyOptions(m, config, opts); err != nil {
		return nil, err
	}

	var bundleFor *string
	if options.isBundleSecret {
		bundleFor = pointer.String(strings.TrimSuffix(config.GetName(), nameSuffixBundle))
	}

	objectMeta, err := ObjectMeta(m.namespace, config, m.lastRotationInitiationTimes[config.GetName()], options.signingCAChecksum, &options.Persist, bundleFor)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{}
	if err := m.client.Get(ctx, kutil.Key(objectMeta.Namespace, objectMeta.Name), secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		secret, err = m.generateAndCreate(ctx, config, objectMeta)
		if err != nil {
			return nil, err
		}
	}

	if !options.isBundleSecret {
		if err := m.addToStore(config.GetName(), secret, current); err != nil {
			return nil, err
		}

		if !options.IgnoreOldSecrets && options.RotationStrategy == KeepOld {
			if err := m.storeOldSecrets(ctx, config.GetName(), secret.Name); err != nil {
				return nil, err
			}
		}

		if err := m.generateBundleSecret(ctx, config); err != nil {
			return nil, err
		}
	}

	if err := m.reconcileSecret(ctx, secret, objectMeta.Labels); err != nil {
		return nil, err
	}

	return secret, nil
}

func (m *manager) generateAndCreate(ctx context.Context, config secretutils.ConfigInterface, objectMeta metav1.ObjectMeta) (*corev1.Secret, error) {
	data, err := config.Generate()
	if err != nil {
		return nil, err
	}

	// For backwards-compatibility, we need to keep some of the existing secrets (cluster-admin token, basic auth
	// password, etc.).
	// TODO(rfranzke): Remove this code in the future
	dataMap, err := m.keepExistingSecretsIfNeeded(ctx, config.GetName(), data.SecretData())
	if err != nil {
		return nil, err
	}

	secret := Secret(objectMeta, dataMap)
	if err := m.client.Create(ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, err
		}

		if err := m.client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
			return nil, err
		}
	}

	m.logger.Info("Generated new secret", "configName", config.GetName(), "secretName", secret.Name)
	return secret, nil
}

func (m *manager) keepExistingSecretsIfNeeded(ctx context.Context, configName string, newData map[string][]byte) (map[string][]byte, error) {
	existingSecret := &corev1.Secret{}

	switch configName {
	case "kube-apiserver-basic-auth":
		if err := m.client.Get(ctx, kutil.Key(m.namespace, "kube-apiserver-basic-auth"), existingSecret); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			return newData, nil
		}

		existingBasicAuth, err := secretutils.LoadBasicAuthFromCSV("", existingSecret.Data[secretutils.DataKeyCSV])
		if err != nil {
			return nil, err
		}
		newBasicAuth, err := secretutils.LoadBasicAuthFromCSV("", newData[secretutils.DataKeyCSV])
		if err != nil {
			return nil, err
		}

		newBasicAuth.Password = existingBasicAuth.Password
		return newBasicAuth.SecretData(), nil

	case "kube-apiserver-static-token":
		if err := m.client.Get(ctx, kutil.Key(m.namespace, "static-token"), existingSecret); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			return newData, nil
		}

		existingStaticToken, err := secretutils.LoadStaticTokenFromCSV("", existingSecret.Data[secretutils.DataKeyStaticTokenCSV])
		if err != nil {
			return nil, err
		}
		newStaticToken, err := secretutils.LoadStaticTokenFromCSV("", newData[secretutils.DataKeyStaticTokenCSV])
		if err != nil {
			return nil, err
		}

		for i, token := range newStaticToken.Tokens {
			for _, existingToken := range existingStaticToken.Tokens {
				if existingToken.Username == token.Username {
					newStaticToken.Tokens[i].Token = existingToken.Token
					break
				}
			}
		}
		return newStaticToken.SecretData(), nil
	}

	return newData, nil
}

func (m *manager) storeOldSecrets(ctx context.Context, name, currentSecretName string) error {
	secretList := &corev1.SecretList{}
	if err := m.client.List(ctx, secretList, client.InNamespace(m.namespace), client.MatchingLabels{
		LabelKeyName:      name,
		LabelKeyManagedBy: LabelValueSecretsManager,
	}); err != nil {
		return err
	}

	var oldSecret *corev1.Secret

	for _, secret := range secretList.Items {
		if secret.Name == currentSecretName {
			continue
		}

		if oldSecret == nil || oldSecret.CreationTimestamp.Time.Before(secret.CreationTimestamp.Time) {
			oldSecret = secret.DeepCopy()
		}
	}

	if oldSecret == nil {
		return nil
	}

	return m.addToStore(oldSecret.Labels[LabelKeyName], oldSecret, old)
}

func (m *manager) generateBundleSecret(ctx context.Context, config secretutils.ConfigInterface) error {
	var bundleConfig secretutils.ConfigInterface

	secrets, found := m.getFromStore(config.GetName())
	if !found {
		return fmt.Errorf("secrets for name %q not found in internal store", config.GetName())
	}

	switch c := config.(type) {
	case *secretutils.CertificateSecretConfig:
		if c.SigningCA == nil {
			certs := [][]byte{secrets.current.obj.Data[secretutils.DataKeyCertificateCA]}
			if secrets.old != nil {
				certs = append(certs, secrets.old.obj.Data[secretutils.DataKeyCertificateCA])
			}

			bundleConfig = &secretutils.CertificateBundleSecretConfig{
				Name:            config.GetName() + nameSuffixBundle,
				CertificatePEMs: certs,
			}
		}
	}

	if bundleConfig == nil {
		return nil
	}

	secret, err := m.Generate(ctx, bundleConfig, isBundleSecret())
	if err != nil {
		return err
	}

	return m.addToStore(config.GetName(), secret, bundle)
}

func (m *manager) reconcileSecret(ctx context.Context, secret *corev1.Secret, labels map[string]string) error {
	patch := client.MergeFrom(secret.DeepCopy())

	var mustPatch bool

	if secret.Immutable == nil || !*secret.Immutable {
		secret.Immutable = pointer.Bool(true)
		mustPatch = true
	}

	for k, v := range labels {
		if secret.Labels[k] != v {
			metav1.SetMetaDataLabel(&secret.ObjectMeta, k, v)
			mustPatch = true
		}
	}

	if !mustPatch {
		return nil
	}

	return m.client.Patch(ctx, secret, patch)
}

// GenerateOption is some configuration that modifies options for a Generate request.
type GenerateOption func(Interface, secretutils.ConfigInterface, *GenerateOptions) error

// GenerateOptions are options for Generate calls.
type GenerateOptions struct {
	// Persist specifies whether the 'persist=true' label should be added to the secret resources.
	Persist bool
	// RotationStrategy specifies how the secret should be rotated in case it needs to get rotated.
	RotationStrategy rotationStrategy
	// IgnoreOldSecrets specifies whether old secrets should be loaded to the internal store.
	IgnoreOldSecrets bool

	signingCAChecksum *string
	isBundleSecret    bool
}

type rotationStrategy string

const (
	// InPlace is a constant for a rotation strategy regenerating a secret and NOT keeping the old one in the system.
	InPlace rotationStrategy = "inplace"
	// KeepOld is a constant for a rotation strategy regenerating a secret and keeping the old one in the system.
	KeepOld rotationStrategy = "keepold"
)

// ApplyOptions applies the given update options on these options, and then returns itself (for convenient chaining).
func (o *GenerateOptions) ApplyOptions(manager Interface, configInterface secretutils.ConfigInterface, opts []GenerateOption) error {
	for _, opt := range opts {
		if err := opt(manager, configInterface, o); err != nil {
			return err
		}
	}
	return nil
}

// SignedByCA returns a function which sets the 'SigningCA' field in case the ConfigInterface provided to the
// Generate request is a CertificateSecretConfig. Additionally, in such case it stores a checksum of the signing
// CA in the options.
func SignedByCA(name string) GenerateOption {
	return func(m Interface, config secretutils.ConfigInterface, options *GenerateOptions) error {
		mgr, ok := m.(*manager)
		if !ok {
			return nil
		}

		certificateConfig, ok := config.(*secretutils.CertificateSecretConfig)
		if !ok {
			return fmt.Errorf("could not apply option to %T, expected *secrets.CertificateSecretConfig", config)
		}

		secrets, found := mgr.getFromStore(name)
		if !found {
			return fmt.Errorf("secrets for name %q not found in internal store", name)
		}

		// Client certificates are always renewed immediately (hence, signed with the current CA), while server
		// certificates are signed with the old CA until they don't exist anymore in the internal store.
		secret := secrets.current
		if certificateConfig.CertType == secretutils.ServerCert && secrets.old != nil {
			secret = *secrets.old
		}

		ca, err := secretutils.LoadCertificate(name, secret.obj.Data[secretutils.DataKeyPrivateKeyCA], secret.obj.Data[secretutils.DataKeyCertificateCA])
		if err != nil {
			return err
		}

		certificateConfig.SigningCA = ca
		options.signingCAChecksum = pointer.String(kutil.TruncateLabelValue(secret.dataChecksum))
		return nil
	}
}

// Persist returns a function which sets the 'Persist' field to true.
func Persist() GenerateOption {
	return func(_ Interface, _ secretutils.ConfigInterface, options *GenerateOptions) error {
		options.Persist = true
		return nil
	}
}

// Rotate returns a function which sets the 'RotationStrategy' field to the specified value.
func Rotate(strategy rotationStrategy) GenerateOption {
	return func(_ Interface, _ secretutils.ConfigInterface, options *GenerateOptions) error {
		options.RotationStrategy = strategy
		return nil
	}
}

// IgnoreOldSecrets returns a function which sets the 'IgnoreOldSecrets' field to true.
func IgnoreOldSecrets() GenerateOption {
	return func(_ Interface, _ secretutils.ConfigInterface, options *GenerateOptions) error {
		options.IgnoreOldSecrets = true
		return nil
	}
}

func isBundleSecret() GenerateOption {
	return func(_ Interface, _ secretutils.ConfigInterface, options *GenerateOptions) error {
		options.isBundleSecret = true
		return nil
	}
}
