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
	"strconv"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
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

	objectMeta, err := ObjectMeta(
		m.namespace,
		m.identity,
		config,
		options.IgnoreConfigChecksumForCASecretName,
		m.lastRotationInitiationTimes[config.GetName()],
		options.signingCAChecksum,
		&options.Persist,
		bundleFor,
	)
	if err != nil {
		return nil, err
	}
	desiredLabels := utils.MergeStringMaps(objectMeta.Labels) // copy labels map

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

	if err := m.maintainLifetimeLabels(config, secret, desiredLabels, options.Validity); err != nil {
		return nil, err
	}

	if !options.isBundleSecret {
		if err := m.addToStore(config.GetName(), secret, current); err != nil {
			return nil, err
		}

		if ignore, err := m.shouldIgnoreOldSecrets(desiredLabels[LabelKeyIssuedAtTime], options); err != nil {
			return nil, err
		} else if !ignore {
			if err := m.storeOldSecrets(ctx, config.GetName(), secret.Name); err != nil {
				return nil, err
			}
		}

		if err := m.generateBundleSecret(ctx, config); err != nil {
			return nil, err
		}
	}

	if err := m.reconcileSecret(ctx, secret, desiredLabels); err != nil {
		return nil, err
	}

	return secret, nil
}

func (m *manager) generateAndCreate(ctx context.Context, config secretutils.ConfigInterface, objectMeta metav1.ObjectMeta) (*corev1.Secret, error) {
	// Use secret name as common name to make sure the x509 subject names in the CA certificates are always unique.
	if certConfig := certificateSecretConfig(config); certConfig != nil && certConfig.CertType == secretutils.CACert {
		certConfig.CommonName = objectMeta.Name
	}

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
	case "kube-apiserver-etcd-encryption-key":
		if err := m.client.Get(ctx, kutil.Key(m.namespace, "etcd-encryption-secret"), existingSecret); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			return newData, nil
		}

		scheme := runtime.NewScheme()
		if err := apiserverconfigv1.AddToScheme(scheme); err != nil {
			return nil, err
		}

		ser := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
		versions := schema.GroupVersions([]schema.GroupVersion{apiserverconfigv1.SchemeGroupVersion})
		codec := serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)

		encryptionConfiguration := &apiserverconfigv1.EncryptionConfiguration{}
		if _, _, err := codec.Decode(existingSecret.Data["encryption-configuration.yaml"], nil, encryptionConfiguration); err != nil {
			return nil, err
		}

		var existingEncryptionKey, existingEncryptionSecret []byte

		if len(encryptionConfiguration.Resources) != 0 {
			for _, provider := range encryptionConfiguration.Resources[0].Providers {
				if provider.AESCBC != nil && len(provider.AESCBC.Keys) != 0 {
					existingEncryptionKey = []byte(provider.AESCBC.Keys[0].Name)
					existingEncryptionSecret = []byte(provider.AESCBC.Keys[0].Secret)
					break
				}
			}
		}

		if existingEncryptionKey == nil || existingEncryptionSecret == nil {
			return nil, fmt.Errorf("old etcd encryption key or secret was not found")
		}

		return map[string][]byte{
			secretutils.DataKeyEncryptionKeyName: existingEncryptionKey,
			secretutils.DataKeyEncryptionSecret:  existingEncryptionSecret,
		}, nil

	case "service-account-key":
		if err := m.client.Get(ctx, kutil.Key(m.namespace, "service-account-key"), existingSecret); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			return newData, nil
		}

		return existingSecret.Data, nil
	}

	return newData, nil
}

func (m *manager) shouldIgnoreOldSecrets(issuedAt string, options *GenerateOptions) (bool, error) {
	// unconditionally ignore old secrets
	if options.RotationStrategy != KeepOld || options.IgnoreOldSecrets {
		return true, nil
	}

	// ignore old secrets if current secret is older than IgnoreOldSecretsAfter
	if options.IgnoreOldSecretsAfter != nil {
		if issuedAt == "" {
			// should never happen
			return false, nil
		}

		issuedAtUnix, err := strconv.ParseInt(issuedAt, 10, 64)
		if err != nil {
			return false, err
		}

		age := m.clock.Now().UTC().Sub(time.Unix(issuedAtUnix, 0).UTC())
		if age >= *options.IgnoreOldSecretsAfter {
			return true, nil
		}
	}

	return false, nil
}

func (m *manager) storeOldSecrets(ctx context.Context, name, currentSecretName string) error {
	secretList := &corev1.SecretList{}
	if err := m.client.List(ctx, secretList, client.InNamespace(m.namespace), client.MatchingLabels{
		LabelKeyName:            name,
		LabelKeyManagedBy:       LabelValueSecretsManager,
		LabelKeyManagerIdentity: m.identity,
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

	case *secretutils.RSASecretConfig:
		if !c.UsedForSSH {
			keys := [][]byte{secrets.current.obj.Data[secretutils.DataKeyRSAPrivateKey]}
			if secrets.old != nil {
				keys = append(keys, secrets.old.obj.Data[secretutils.DataKeyRSAPrivateKey])
			}

			bundleConfig = &secretutils.RSAPrivateKeyBundleSecretConfig{
				Name:           config.GetName() + nameSuffixBundle,
				PrivateKeyPEMs: keys,
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

func (m *manager) maintainLifetimeLabels(
	config secretutils.ConfigInterface,
	secret *corev1.Secret,
	desiredLabels map[string]string,
	validity time.Duration,
) error {
	issuedAt := secret.Labels[LabelKeyIssuedAtTime]
	if issuedAt == "" {
		issuedAt = unixTime(m.clock.Now())
	}
	desiredLabels[LabelKeyIssuedAtTime] = issuedAt

	if validity > 0 {
		desiredLabels[LabelKeyValidUntilTime] = unixTime(m.clock.Now().Add(validity))

		// Handle changed validity values in case there already is a valid-until-time label from previous Generate
		// invocations.
		if secret.Labels[LabelKeyValidUntilTime] != "" {
			issuedAtTime, err := strconv.ParseInt(issuedAt, 10, 64)
			if err != nil {
				return err
			}

			existingValidUntilTime, err := strconv.ParseInt(secret.Labels[LabelKeyValidUntilTime], 10, 64)
			if err != nil {
				return err
			}

			if oldValidity := time.Duration(existingValidUntilTime - issuedAtTime); oldValidity != validity {
				desiredLabels[LabelKeyValidUntilTime] = unixTime(time.Unix(issuedAtTime, 0).UTC().Add(validity))
				// If this has yielded a valid-until-time which is in the past then the next instantiation of the
				// secrets manager will regenerate the secret since it has expired.
			}
		}
	}

	var dataKeyCertificate string
	switch cfg := config.(type) {
	case *secretutils.CertificateSecretConfig:
		dataKeyCertificate = secretutils.DataKeyCertificate
		if cfg.CertType == secretutils.CACert {
			dataKeyCertificate = secretutils.DataKeyCertificateCA
		}
	case *secretutils.ControlPlaneSecretConfig:
		if cfg.CertificateSecretConfig == nil {
			return nil
		}
		dataKeyCertificate = secretutils.ControlPlaneSecretDataKeyCertificatePEM(config.GetName())
	default:
		return nil
	}

	certificate, err := utils.DecodeCertificate(secret.Data[dataKeyCertificate])
	if err != nil {
		return fmt.Errorf("error decoding certificate when trying to maintain lifetime labels: %w", err)
	}

	desiredLabels[LabelKeyIssuedAtTime] = unixTime(certificate.NotBefore)
	desiredLabels[LabelKeyValidUntilTime] = unixTime(certificate.NotAfter)
	return nil
}

func (m *manager) reconcileSecret(ctx context.Context, secret *corev1.Secret, labels map[string]string) error {
	patch := client.MergeFrom(secret.DeepCopy())

	var mustPatch bool

	if secret.Immutable == nil || !*secret.Immutable {
		secret.Immutable = pointer.Bool(true)
		mustPatch = true
	}

	// Check if desired labels must be added or changed.
	for k, desired := range labels {
		if current, ok := secret.Labels[k]; !ok || current != desired {
			metav1.SetMetaDataLabel(&secret.ObjectMeta, k, desired)
			mustPatch = true
		}
	}

	// Check if existing labels must be removed
	for k := range secret.Labels {
		if _, ok := labels[k]; !ok {
			delete(secret.Labels, k)
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
	// IgnoreOldSecrets specifies whether old secrets should be dropped.
	IgnoreOldSecrets bool
	// IgnoreOldSecretsAfter specifies that old secrets should be dropped once a given duration after rotation has passed.
	IgnoreOldSecretsAfter *time.Duration
	// Validity specifies for how long the secret should be valid.
	Validity time.Duration
	// IgnoreConfigChecksumForCASecretName specifies whether the secret config checksum should be ignored when
	// computing the secret name for CA secrets.
	IgnoreConfigChecksumForCASecretName bool

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

// SignedByCAOption is some configuration that modifies options for a SignedByCA request.
type SignedByCAOption interface {
	// ApplyToOptions applies this configuration to the given options.
	ApplyToOptions(*SignedByCAOptions)
}

// SignedByCAOptions are options for SignedByCA calls.
type SignedByCAOptions struct {
	// CAClass specifies which CA should be used to sign the requested certificate. Server certificates are signed with
	// the old CA by default, however one might want to use the current CA instead. Similarly, client certificates are
	// signed with the current CA by default, however one might want to use the old CA instead.
	CAClass *secretClass
}

// ApplyOptions applies the given update options on these options, and then returns itself (for convenient chaining).
func (o *SignedByCAOptions) ApplyOptions(opts []SignedByCAOption) *SignedByCAOptions {
	for _, opt := range opts {
		opt.ApplyToOptions(o)
	}
	return o
}

var (
	// UseCurrentCA sets the CAClass field to 'current' in the SignedByCAOptions.
	UseCurrentCA = useCAClassOption{current}
	// UseOldCA sets the CAClass field to 'old' in the SignedByCAOptions.
	UseOldCA = useCAClassOption{old}
)

type useCAClassOption struct {
	class secretClass
}

func (o useCAClassOption) ApplyToOptions(options *SignedByCAOptions) {
	options.CAClass = &o.class
}

// SignedByCA returns a function which sets the 'SigningCA' field in case the ConfigInterface provided to the
// Generate request is a CertificateSecretConfig. Additionally, in such case it stores a checksum of the signing
// CA in the options.
func SignedByCA(name string, opts ...SignedByCAOption) GenerateOption {
	signedByCAOptions := &SignedByCAOptions{}
	signedByCAOptions.ApplyOptions(opts)

	return func(m Interface, config secretutils.ConfigInterface, options *GenerateOptions) error {
		mgr, ok := m.(*manager)
		if !ok {
			return nil
		}

		certificateConfig := certificateSecretConfig(config)
		if certificateConfig == nil {
			return fmt.Errorf("could not apply option to %T, expected *secrets.CertificateSecretConfig", config)
		}

		secrets, found := mgr.getFromStore(name)
		if !found {
			return fmt.Errorf("secrets for name %q not found in internal store", name)
		}

		secret := secrets.current
		switch certificateConfig.CertType {
		case secretutils.ClientCert:
			// Client certificates are signed with the current CA by default unless the CAClass option was overwritten.
			if signedByCAOptions.CAClass != nil && *signedByCAOptions.CAClass == old && secrets.old != nil {
				secret = *secrets.old
			}

		case secretutils.ServerCert, secretutils.ServerClientCert:
			// Server certificates are signed with the old CA by default (if it exists) unless the CAClass option was
			// overwritten.
			if secrets.old != nil && (signedByCAOptions.CAClass == nil || *signedByCAOptions.CAClass != current) {
				secret = *secrets.old
			}
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

// IgnoreOldSecretsAfter returns a function which sets the 'IgnoreOldSecretsAfter' field to the given duration.
func IgnoreOldSecretsAfter(d time.Duration) GenerateOption {
	return func(_ Interface, _ secretutils.ConfigInterface, options *GenerateOptions) error {
		options.IgnoreOldSecretsAfter = &d
		return nil
	}
}

// Validity returns a function which sets the 'Validity' field to the provided value. Note that the value is ignored in
// case Generate is called with a certificate secret configuration.
func Validity(v time.Duration) GenerateOption {
	return func(_ Interface, _ secretutils.ConfigInterface, options *GenerateOptions) error {
		options.Validity = v
		return nil
	}
}

// IgnoreConfigChecksumForCASecretName returns a function which sets the 'IgnoreConfigChecksumForCASecretName' field to
// true.
func IgnoreConfigChecksumForCASecretName() GenerateOption {
	return func(_ Interface, _ secretutils.ConfigInterface, options *GenerateOptions) error {
		options.IgnoreConfigChecksumForCASecretName = true
		return nil
	}
}

func isBundleSecret() GenerateOption {
	return func(_ Interface, _ secretutils.ConfigInterface, options *GenerateOptions) error {
		options.isBundleSecret = true
		return nil
	}
}
