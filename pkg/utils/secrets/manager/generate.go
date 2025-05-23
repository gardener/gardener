// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

func (m *manager) Generate(ctx context.Context, config secretsutils.ConfigInterface, opts ...GenerateOption) (*corev1.Secret, error) {
	options := &GenerateOptions{}
	if err := options.ApplyOptions(m, config, opts); err != nil {
		return nil, fmt.Errorf("failed applying generate options for config %s: %w", config.GetName(), err)
	}

	var bundleFor *string
	if options.isBundleSecret {
		bundleFor = ptr.To(strings.TrimSuffix(config.GetName(), nameSuffixBundle))
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
		return nil, fmt.Errorf("failed computing object metadata for config %s: %w", config.GetName(), err)
	}
	desiredLabels := utils.MergeStringMaps(objectMeta.Labels) // copy labels map

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: objectMeta.Name, Namespace: objectMeta.Namespace}}
	if err := m.client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed reading secret %s for config %s: %w", client.ObjectKeyFromObject(secret), config.GetName(), err)
		}

		secret, err = m.generateAndCreate(ctx, config, objectMeta)
		if err != nil {
			return nil, fmt.Errorf("failed generating and creating new secret %s for config %s: %w", client.ObjectKey{Name: objectMeta.Name, Namespace: objectMeta.Namespace}, config.GetName(), err)
		}
	}

	if err := m.maintainLifetimeLabels(config, secret, desiredLabels, options.Validity, options.RenewAfterValidityPercentage); err != nil {
		return nil, fmt.Errorf("failed maintaining lifetime labels on secret %s for config %s: %w", client.ObjectKeyFromObject(secret), config.GetName(), err)
	}

	if !options.isBundleSecret {
		if err := m.addToStore(config.GetName(), secret, current); err != nil {
			return nil, fmt.Errorf("failed adding current secret %s for config %s to internal store: %w", client.ObjectKeyFromObject(secret), config.GetName(), err)
		}

		if ignore, err := m.shouldIgnoreOldSecrets(desiredLabels[LabelKeyIssuedAtTime], options); err != nil {
			return nil, fmt.Errorf("failed checking whether old secrets should be ignored for config %s: %w", config.GetName(), err)
		} else if !ignore {
			if err := m.storeOldSecrets(ctx, config.GetName(), secret.Name); err != nil {
				return nil, fmt.Errorf("failed adding old secrets for config %s to internal store: %w", config.GetName(), err)
			}
		}

		if err := m.generateBundleSecret(ctx, config); err != nil {
			return nil, fmt.Errorf("failed generating bundle secret for config %s: %w", config.GetName(), err)
		}
	}

	if err := m.reconcileSecret(ctx, secret, desiredLabels); err != nil {
		return nil, fmt.Errorf("failed reconciling existing secret %s for config %s: %w", client.ObjectKeyFromObject(secret), config.GetName(), err)
	}

	return secret, nil
}

func (m *manager) generateAndCreate(ctx context.Context, config secretsutils.ConfigInterface, objectMeta metav1.ObjectMeta) (*corev1.Secret, error) {
	// Use secret name as common name to make sure the x509 subject names in the CA certificates are always unique.
	if certConfig := certificateSecretConfig(config); certConfig != nil && certConfig.CertType == secretsutils.CACert {
		certConfig.CommonName = objectMeta.Name
	}

	data, err := config.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed generating data: %w", err)
	}

	dataMap, err := m.keepExistingSecretsIfNeeded(ctx, config.GetName(), data.SecretData())
	if err != nil {
		return nil, fmt.Errorf("failed taking over data from existing secret when needed: %w", err)
	}

	secret := Secret(objectMeta, dataMap)
	if err := m.client.Create(ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("failed creating new secret: %w", err)
		}

		if err := m.client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
			return nil, fmt.Errorf("failed reading existing secret: %w", err)
		}
	}

	m.logger.Info("Generated new secret", "configName", config.GetName(), "secretName", secret.Name)
	return secret, nil
}

func (m *manager) keepExistingSecretsIfNeeded(ctx context.Context, configName string, newData map[string][]byte) (map[string][]byte, error) {
	existingSecrets := &corev1.SecretList{}
	if err := m.client.List(ctx, existingSecrets, client.InNamespace(m.namespace), client.MatchingLabels{LabelKeyUseDataForName: configName}); err != nil {
		return nil, err
	}

	if len(existingSecrets.Items) > 1 {
		return nil, fmt.Errorf("found more than one existing secret with %q label for config %q", LabelKeyUseDataForName, configName)
	}

	if len(existingSecrets.Items) == 1 {
		return existingSecrets.Items[0].Data, nil
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

func (m *manager) generateBundleSecret(ctx context.Context, config secretsutils.ConfigInterface) error {
	var bundleConfig secretsutils.ConfigInterface

	secrets, found := m.getFromStore(config.GetName())
	if !found {
		return fmt.Errorf("secrets for name %q not found in internal store", config.GetName())
	}

	switch c := config.(type) {
	case *secretsutils.CertificateSecretConfig:
		if c.SigningCA == nil {
			certs := [][]byte{secrets.current.obj.Data[secretsutils.DataKeyCertificateCA]}
			if secrets.old != nil {
				valid, err := m.isStillValid(secrets.old.obj)
				if err != nil {
					return fmt.Errorf("failed validating old secret: %w", err)
				}
				if valid {
					certs = append(certs, secrets.old.obj.Data[secretsutils.DataKeyCertificateCA])
				}
			}

			bundleConfig = &secretsutils.CertificateBundleSecretConfig{
				Name:            config.GetName() + nameSuffixBundle,
				CertificatePEMs: certs,
			}
		}

	case *secretsutils.RSASecretConfig:
		if !c.UsedForSSH {
			keys := [][]byte{secrets.current.obj.Data[secretsutils.DataKeyRSAPrivateKey]}
			if secrets.old != nil {
				keys = append(keys, secrets.old.obj.Data[secretsutils.DataKeyRSAPrivateKey])
			}

			bundleConfig = &secretsutils.RSAPrivateKeyBundleSecretConfig{
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

func (m *manager) isStillValid(secret *corev1.Secret) (bool, error) {
	value := secret.Labels[LabelKeyValidUntilTime]
	if value == "" {
		return true, nil
	}
	validUntilUnix, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parsing label %q of secret %s failed: %w", LabelKeyValidUntilTime, client.ObjectKeyFromObject(secret), err)
	}
	return m.clock.Now().UTC().Unix() < validUntilUnix, nil
}

func (m *manager) maintainLifetimeLabels(
	config secretsutils.ConfigInterface,
	secret *corev1.Secret,
	desiredLabels map[string]string,
	validity time.Duration,
	renewAfterValidityPercentage int,
) error {
	issuedAt := secret.Labels[LabelKeyIssuedAtTime]
	if issuedAt == "" {
		issuedAt = unixTime(m.clock.Now())
	}
	desiredLabels[LabelKeyIssuedAtTime] = issuedAt
	if renewAfterValidityPercentage > 0 {
		desiredLabels[LabelKeyRenewAfterValidityPercentage] = fmt.Sprintf("%d", renewAfterValidityPercentage)
	}

	if validity > 0 {
		desiredLabels[LabelKeyValidUntilTime] = unixTime(m.clock.Now().Add(validity))

		// Handle changed validity values in case there already is a valid-until-time label from previous Generate
		// invocations.
		if secret.Labels[LabelKeyValidUntilTime] != "" {
			issuedAtTime, err := strconv.ParseInt(issuedAt, 10, 64)
			if err != nil {
				return fmt.Errorf("failed converting %s to int64: %w", issuedAt, err)
			}

			existingValidUntilTime, err := strconv.ParseInt(secret.Labels[LabelKeyValidUntilTime], 10, 64)
			if err != nil {
				return fmt.Errorf("failed converting %s from label %s to int64: %w", secret.Labels[LabelKeyValidUntilTime], LabelKeyValidUntilTime, err)
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
	case *secretsutils.CertificateSecretConfig:
		dataKeyCertificate = secretsutils.DataKeyCertificate
		if cfg.CertType == secretsutils.CACert {
			dataKeyCertificate = secretsutils.DataKeyCertificateCA
		}
	case *secretsutils.ControlPlaneSecretConfig:
		if cfg.CertificateSecretConfig == nil {
			return nil
		}
		dataKeyCertificate = secretsutils.ControlPlaneSecretDataKeyCertificatePEM(config.GetName())
	default:
		return nil
	}

	certificate, err := utils.DecodeCertificate(secret.Data[dataKeyCertificate])
	if err != nil {
		return fmt.Errorf("error decoding certificate from data key %s: %w", dataKeyCertificate, err)
	}

	desiredLabels[LabelKeyIssuedAtTime] = unixTime(certificate.NotBefore)
	desiredLabels[LabelKeyValidUntilTime] = unixTime(certificate.NotAfter)
	return nil
}

func (m *manager) reconcileSecret(ctx context.Context, secret *corev1.Secret, labels map[string]string) error {
	patch := client.MergeFrom(secret.DeepCopy())

	var mustPatch bool

	if secret.Immutable == nil || !*secret.Immutable {
		secret.Immutable = ptr.To(true)
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
type GenerateOption func(Interface, secretsutils.ConfigInterface, *GenerateOptions) error

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
	// RenewAfterValidityPercentage sets the percentage of the validity when the certificate should be renewed.
	// The effective check for renewal is after the given percentage of validity or 10d before the end of validity.
	// Zero value means the default percentage is used (80%).
	RenewAfterValidityPercentage int
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
func (o *GenerateOptions) ApplyOptions(manager Interface, configInterface secretsutils.ConfigInterface, opts []GenerateOption) error {
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

	return func(m Interface, config secretsutils.ConfigInterface, options *GenerateOptions) error {
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
		case secretsutils.ClientCert:
			// Client certificates are signed with the current CA by default unless the CAClass option was overwritten.
			if signedByCAOptions.CAClass != nil && *signedByCAOptions.CAClass == old && secrets.old != nil {
				secret = *secrets.old
			}

		case secretsutils.ServerCert, secretsutils.ServerClientCert:
			// Server certificates are signed with the old CA by default (if it exists) unless the CAClass option was
			// overwritten.
			if secrets.old != nil && (signedByCAOptions.CAClass == nil || *signedByCAOptions.CAClass != current) {
				secret = *secrets.old
			}
		}

		ca, err := secretsutils.LoadCertificate(name, secret.obj.Data[secretsutils.DataKeyPrivateKeyCA], secret.obj.Data[secretsutils.DataKeyCertificateCA])
		if err != nil {
			return err
		}

		certificateConfig.SigningCA = ca
		options.signingCAChecksum = ptr.To(kubernetesutils.TruncateLabelValue(secret.dataChecksum))
		return nil
	}
}

// Persist returns a function which sets the 'Persist' field to true.
func Persist() GenerateOption {
	return func(_ Interface, _ secretsutils.ConfigInterface, options *GenerateOptions) error {
		options.Persist = true
		return nil
	}
}

// Rotate returns a function which sets the 'RotationStrategy' field to the specified value.
func Rotate(strategy rotationStrategy) GenerateOption {
	return func(_ Interface, _ secretsutils.ConfigInterface, options *GenerateOptions) error {
		options.RotationStrategy = strategy
		return nil
	}
}

// IgnoreOldSecrets returns a function which sets the 'IgnoreOldSecrets' field to true.
func IgnoreOldSecrets() GenerateOption {
	return func(_ Interface, _ secretsutils.ConfigInterface, options *GenerateOptions) error {
		options.IgnoreOldSecrets = true
		return nil
	}
}

// IgnoreOldSecretsAfter returns a function which sets the 'IgnoreOldSecretsAfter' field to the given duration.
func IgnoreOldSecretsAfter(d time.Duration) GenerateOption {
	return func(_ Interface, _ secretsutils.ConfigInterface, options *GenerateOptions) error {
		options.IgnoreOldSecretsAfter = &d
		return nil
	}
}

// Validity returns a function which sets the 'Validity' field to the provided value. Note that the value is ignored in
// case Generate is called with a certificate secret configuration.
func Validity(v time.Duration) GenerateOption {
	return func(_ Interface, _ secretsutils.ConfigInterface, options *GenerateOptions) error {
		options.Validity = v
		return nil
	}
}

// RenewAfterValidityPercentage returns a function which sets the 'RenewAfterValidityPercentage' field to the provided
// value.
func RenewAfterValidityPercentage(v int) GenerateOption {
	return func(_ Interface, _ secretsutils.ConfigInterface, options *GenerateOptions) error {
		options.RenewAfterValidityPercentage = v
		return nil
	}
}

// IgnoreConfigChecksumForCASecretName returns a function which sets the 'IgnoreConfigChecksumForCASecretName' field to
// true.
func IgnoreConfigChecksumForCASecretName() GenerateOption {
	return func(_ Interface, _ secretsutils.ConfigInterface, options *GenerateOptions) error {
		options.IgnoreConfigChecksumForCASecretName = true
		return nil
	}
}

func isBundleSecret() GenerateOption {
	return func(_ Interface, _ secretsutils.ConfigInterface, options *GenerateOptions) error {
		options.isBundleSecret = true
		return nil
	}
}
