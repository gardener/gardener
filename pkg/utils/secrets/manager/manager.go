// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/mitchellh/hashstructure/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	// LabelKeyName is a constant for a key of a label on a Secret describing the name.
	LabelKeyName = "name"
	// LabelKeyManagedBy is a constant for a key of a label on a Secret describing who is managing it.
	LabelKeyManagedBy = "managed-by"
	// LabelKeyManagerIdentity is a constant for a key of a label on a Secret describing which secret manager instance
	// is managing it.
	LabelKeyManagerIdentity = "manager-identity"
	// LabelKeyChecksumConfig is a constant for a key of a label on a Secret describing the checksum of the
	// configuration used to create the data.
	LabelKeyChecksumConfig = "checksum-of-config"
	// LabelKeyChecksumSigningCA is a constant for a key of a label on a Secret describing the checksum of the
	// certificate authority which has signed the client or server certificate in the data.
	LabelKeyChecksumSigningCA = "checksum-of-signing-ca"
	// LabelKeyBundleFor is a constant for a key of a label on a Secret describing that it is a bundle secret for
	// another secret.
	LabelKeyBundleFor = "bundle-for"
	// LabelKeyPersist is a constant for a key of a label on a Secret describing that it should get persisted.
	LabelKeyPersist = "persist"
	// LabelKeyLastRotationInitiationTime is a constant for a key of a label on a Secret describing the unix timestamps
	// of when the last secret rotation was initiated.
	LabelKeyLastRotationInitiationTime = "last-rotation-initiation-time"
	// LabelKeyIssuedAtTime is a constant for a key of a label on a Secret describing the time of when the secret data
	// was created. In case the data contains a certificate it is the time part of the certificate's 'not before' field.
	LabelKeyIssuedAtTime = "issued-at-time"
	// LabelKeyValidUntilTime is a constant for a key of a label on a Secret describing the time of how long the secret
	// data is valid. In case the data contains a certificate it is the time part of the certificate's 'not after'
	// field.
	LabelKeyValidUntilTime = "valid-until-time"
	// LabelKeyRenewAfterValidityPercentage is a constant for a key of a label on a certificate secret describing the
	// percentage of the validity when the certificate should be renewed. The effective check for renewal is after the
	// given percentage of validity or 10d before the end of validity. If not specified the default percentage is 80.
	LabelKeyRenewAfterValidityPercentage = "renew-after-validity-percentage"
	// LabelKeyUseDataForName is a constant for a key of a label on a Secret describing that its data should be used
	// instead of generating a fresh secret with the same name.
	LabelKeyUseDataForName = "secrets-manager-use-data-for-name"

	// LabelValueTrue is a constant for a value of a label on a Secret describing the value 'true'.
	LabelValueTrue = "true"
	// LabelValueSecretsManager is a constant for a value of a label on a Secret describing the value 'secret-manager'.
	LabelValueSecretsManager = "secrets-manager"

	nameSuffixBundle = "-bundle"
)

type (
	manager struct {
		lock                        sync.Mutex
		clock                       clock.Clock
		store                       secretStore
		logger                      logr.Logger
		client                      client.Client
		namespace                   string
		identity                    string
		lastRotationInitiationTimes nameToUnixTime
	}

	nameToUnixTime map[string]string

	secretStore map[string]secretInfos
	secretInfos struct {
		current secretInfo
		old     *secretInfo
		bundle  *secretInfo
	}
	secretInfo struct {
		obj                        *corev1.Secret
		dataChecksum               string
		lastRotationInitiationTime int64
	}

	// Config specifies certain configuration options for the manager.
	Config struct {
		// CASecretAutoRotation states whether CA secrets are considered for automatic rotation (defaults to false).
		CASecretAutoRotation bool
		// SecretNamesToTimes is a map whose keys are secret names and whose values are the last rotation initiation
		// times.
		SecretNamesToTimes map[string]time.Time
	}
)

var _ Interface = &manager{}

type secretClass string

const (
	current secretClass = "current"
	old     secretClass = "old"
	bundle  secretClass = "bundle"
)

// New returns a new manager for secrets in a given namespace.
func New(
	ctx context.Context,
	logger logr.Logger,
	clock clock.Clock,
	c client.Client,
	namespace string,
	identity string,
	rotation Config,
) (
	Interface,
	error,
) {
	m := &manager{
		store:                       make(secretStore),
		clock:                       clock,
		logger:                      logger.WithValues("namespace", namespace),
		client:                      c,
		namespace:                   namespace,
		identity:                    identity,
		lastRotationInitiationTimes: make(nameToUnixTime),
	}

	if err := m.initialize(ctx, rotation); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *manager) listSecrets(ctx context.Context) (*corev1.SecretList, error) {
	secretList := &corev1.SecretList{}
	return secretList, m.client.List(ctx, secretList, client.InNamespace(m.namespace), client.MatchingLabels{
		LabelKeyManagedBy:       LabelValueSecretsManager,
		LabelKeyManagerIdentity: m.identity,
	})
}

func (m *manager) initialize(ctx context.Context, rotation Config) error {
	secretList, err := m.listSecrets(ctx)
	if err != nil {
		return err
	}

	nameToNewestSecret := make(map[string]corev1.Secret, len(secretList.Items))

	// Find the newest secret in system for the respective secret names. Read their existing
	// last-rotation-initiation-time labels and store them in our internal map.
	for _, secret := range secretList.Items {
		oldSecret, found := nameToNewestSecret[secret.Labels[LabelKeyName]]
		if !found || oldSecret.CreationTimestamp.Time.Before(secret.CreationTimestamp.Time) {
			nameToNewestSecret[secret.Labels[LabelKeyName]] = *secret.DeepCopy()
			m.lastRotationInitiationTimes[secret.Labels[LabelKeyName]] = secret.Labels[LabelKeyLastRotationInitiationTime]
		}
	}

	// Check if the secrets must be automatically renewed because they are about to expire.
	for name, secret := range nameToNewestSecret {
		if isCASecret(secret.Data) && !rotation.CASecretAutoRotation {
			continue
		}

		mustRenew, err := m.mustAutoRenewSecret(secret)
		if err != nil {
			return err
		}

		if mustRenew {
			m.logger.Info("Preparing secret for automatic renewal", "secret", secret.Name, "issuedAt", secret.Labels[LabelKeyIssuedAtTime], "validUntil", secret.Labels[LabelKeyValidUntilTime])
			m.lastRotationInitiationTimes[name] = unixTime(m.clock.Now())
		}
	}

	// If the user has provided last rotation initiation times then use those.
	for name, time := range rotation.SecretNamesToTimes {
		m.lastRotationInitiationTimes[name] = unixTime(time)
	}

	return nil
}

func (m *manager) mustAutoRenewSecret(secret corev1.Secret) (bool, error) {
	if secret.Labels[LabelKeyIssuedAtTime] == "" || secret.Labels[LabelKeyValidUntilTime] == "" {
		return false, nil
	}

	issuedAtUnix, err := strconv.ParseInt(secret.Labels[LabelKeyIssuedAtTime], 10, 64)
	if err != nil {
		return false, err
	}

	validUntilUnix, err := strconv.ParseInt(secret.Labels[LabelKeyValidUntilTime], 10, 64)
	if err != nil {
		return false, err
	}

	renewAfterValidityPercentage := 80
	if secret.Labels[LabelKeyRenewAfterValidityPercentage] != "" {
		value, err := strconv.Atoi(secret.Labels[LabelKeyRenewAfterValidityPercentage])
		if err != nil {
			return false, err
		}
		renewAfterValidityPercentage = value
	}

	var (
		validity    = validUntilUnix - issuedAtUnix
		renewAtUnix = issuedAtUnix + validity*int64(renewAfterValidityPercentage)/100
		renewAt     = time.Unix(renewAtUnix, 0).UTC()
		validUntil  = time.Unix(validUntilUnix, 0).UTC()
		now         = m.clock.Now().UTC()
	)

	// Renew if 80% of the validity has been reached or if the secret expires in less than 10d.
	return now.After(renewAt) || now.After(validUntil.Add(-10*24*time.Hour)), nil
}

func (m *manager) addToStore(name string, secret *corev1.Secret, class secretClass) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	info, err := computeSecretInfo(secret)
	if err != nil {
		return err
	}

	secrets := m.store[name]

	switch class {
	case current:
		secrets.current = info
	case old:
		secrets.old = &info
	case bundle:
		secrets.bundle = &info
	}

	m.store[name] = secrets

	return nil
}

func (m *manager) getFromStore(name string) (secretInfos, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	secrets, ok := m.store[name]
	return secrets, ok
}

func computeSecretInfo(obj *corev1.Secret) (secretInfo, error) {
	var (
		lastRotationStartTime int64
		err                   error
	)

	if v := obj.Labels[LabelKeyLastRotationInitiationTime]; len(v) > 0 {
		lastRotationStartTime, err = strconv.ParseInt(obj.Labels[LabelKeyLastRotationInitiationTime], 10, 64)
		if err != nil {
			return secretInfo{}, err
		}
	}

	return secretInfo{
		obj:                        obj,
		dataChecksum:               utils.ComputeSecretChecksum(obj.Data),
		lastRotationInitiationTime: lastRotationStartTime,
	}, nil
}

// ObjectMeta returns the object meta based on the given settings.
func ObjectMeta(
	namespace string,
	managerIdentity string,
	config secretsutils.ConfigInterface,
	ignoreConfigChecksumForCASecretName bool,
	lastRotationInitiationTime string,
	signingCAChecksum *string,
	persist *bool,
	bundleFor *string,
) (
	metav1.ObjectMeta,
	error,
) {
	configHash, err := hashstructure.Hash(config, hashstructure.FormatV2, &hashstructure.HashOptions{IgnoreZeroValue: true})
	if err != nil {
		return metav1.ObjectMeta{}, err
	}

	labels := map[string]string{
		LabelKeyName:                       config.GetName(),
		LabelKeyManagedBy:                  LabelValueSecretsManager,
		LabelKeyManagerIdentity:            managerIdentity,
		LabelKeyChecksumConfig:             strconv.FormatUint(configHash, 10),
		LabelKeyLastRotationInitiationTime: lastRotationInitiationTime,
	}

	if signingCAChecksum != nil {
		labels[LabelKeyChecksumSigningCA] = *signingCAChecksum
	}

	if persist != nil && *persist {
		labels[LabelKeyPersist] = LabelValueTrue
	}

	if bundleFor != nil {
		labels[LabelKeyBundleFor] = *bundleFor
	}

	return metav1.ObjectMeta{
		Name:      computeSecretName(config, labels, ignoreConfigChecksumForCASecretName),
		Namespace: namespace,
		Labels:    labels,
	}, nil
}

func computeSecretName(config secretsutils.ConfigInterface, labels map[string]string, ignoreConfigChecksumForCASecretName bool) string {
	name := config.GetName()

	// For backwards-compatibility, we might need to keep the static names of the CA secrets so that external components
	// (like extensions, etc.) relying on them don't break. This is why it is possible to opt out of the fact that the
	// config checksum is considered for the name computation.
	if cfg, ok := config.(*secretsutils.CertificateSecretConfig); !ok || cfg.SigningCA != nil || !ignoreConfigChecksumForCASecretName {
		if infix := labels[LabelKeyChecksumConfig] + labels[LabelKeyChecksumSigningCA]; len(infix) > 0 {
			name += "-" + utils.ComputeSHA256Hex([]byte(infix))[:8]
		}
	}

	if suffix := labels[LabelKeyLastRotationInitiationTime]; len(suffix) > 0 {
		name += "-" + utils.ComputeSHA256Hex([]byte(suffix))[:5]
	}

	return name
}

// Secret constructs a *corev1.Secret for the given metadata and data.
func Secret(objectMeta metav1.ObjectMeta, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: objectMeta,
		Data:       data,
		Type:       secretTypeForData(data),
		Immutable:  ptr.To(true),
	}
}

func secretTypeForData(data map[string][]byte) corev1.SecretType {
	secretType := corev1.SecretTypeOpaque
	if data[secretsutils.DataKeyCertificate] != nil && data[secretsutils.DataKeyPrivateKey] != nil {
		secretType = corev1.SecretTypeTLS
	}
	return secretType
}

func unixTime(in time.Time) string {
	return strconv.FormatInt(in.UTC().Unix(), 10)
}

func isCASecret(data map[string][]byte) bool {
	return data[secretsutils.DataKeyCertificateCA] != nil && data[secretsutils.DataKeyPrivateKeyCA] != nil
}

func certificateSecretConfig(config secretsutils.ConfigInterface) *secretsutils.CertificateSecretConfig {
	var certificateConfig *secretsutils.CertificateSecretConfig

	switch cfg := config.(type) {
	case *secretsutils.CertificateSecretConfig:
		certificateConfig = cfg
	case *secretsutils.ControlPlaneSecretConfig:
		certificateConfig = cfg.CertificateSecretConfig
	}

	return certificateConfig
}
