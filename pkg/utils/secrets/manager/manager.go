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
	"strconv"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/utils"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LabelKeyName is a constant for a key of a label on a Secret describing the name.
	LabelKeyName = "name"
	// LabelKeyManagedBy is a constant for a key of a label on a Secret describing whose managing it.
	LabelKeyManagedBy = "managed-by"
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
	// LabelKeyLastRotationStartedTime is a constant for a key of a value on a Secret describing the unix timestamps
	// of when the secret was last rotated.
	LabelKeyLastRotationStartedTime = "last-rotation-started-time"

	// LabelValueTrue is a constant for a value of a label on a Secret describing the value 'true'.
	LabelValueTrue = "true"
	// LabelValueSecretsManager is a constant for a value of a label on a Secret describing the value 'secret-manager'.
	LabelValueSecretsManager = "secrets-manager"

	nameSuffixBundle = "-bundle"
)

type (
	manager struct {
		lock                   sync.Mutex
		logger                 logrus.FieldLogger
		client                 client.Client
		namespace              string
		lastRotationStartTimes nameToUnixTime
		store                  secretStore
	}

	nameToUnixTime map[string]string

	secretStore map[string]secretInfos
	secretInfos struct {
		current secretInfo
		old     *secretInfo
		bundle  *secretInfo
	}
	secretInfo struct {
		obj                     *corev1.Secret
		dataChecksum            string
		lastRotationStartedTime int64
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
func New(logger logrus.FieldLogger, client client.Client, namespace string, nameToTime map[string]time.Time) Interface {
	lastRotationStartTimes := make(map[string]string)

	for name, time := range nameToTime {
		lastRotationStartTimes[name] = strconv.FormatInt(time.UTC().Unix(), 10)
	}

	return &manager{
		logger:                 logger,
		client:                 client,
		namespace:              namespace,
		store:                  make(secretStore),
		lastRotationStartTimes: lastRotationStartTimes,
	}
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
		if secrets.old == nil || secrets.old.lastRotationStartedTime < info.lastRotationStartedTime {
			secrets.old = &info
		}
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

	if v := obj.Labels[LabelKeyLastRotationStartedTime]; len(v) > 0 {
		lastRotationStartTime, err = strconv.ParseInt(obj.Labels[LabelKeyLastRotationStartedTime], 10, 64)
		if err != nil {
			return secretInfo{}, err
		}
	}

	return secretInfo{
		obj:                     obj,
		dataChecksum:            utils.ComputeSecretChecksum(obj.Data),
		lastRotationStartedTime: lastRotationStartTime,
	}, nil
}

// ObjectMeta returns the object meta based on the given settings.
func ObjectMeta(
	namespace string,
	config secretutils.ConfigInterface,
	lastRotationStartedTime string,
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
		LabelKeyName:                    config.GetName(),
		LabelKeyManagedBy:               LabelValueSecretsManager,
		LabelKeyChecksumConfig:          strconv.FormatUint(configHash, 10),
		LabelKeyLastRotationStartedTime: lastRotationStartedTime,
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
		Name:      computeSecretName(config, labels),
		Namespace: namespace,
		Labels:    labels,
	}, nil
}

func computeSecretName(config secretutils.ConfigInterface, labels map[string]string) string {
	name := config.GetName()

	// For backwards-compatibility we need to keep the static names of the CA secrets so that components relying on them
	// don't break.
	// TODO(rfranzke): The outer constraint can be removed in the future once we adapted all components relying on the
	//  constant CA secret names, i.e., in this case we can always use 'GenerateName'.
	if cfg, ok := config.(*secretutils.CertificateSecretConfig); !ok || cfg.SigningCA != nil {
		if infix := labels[LabelKeyChecksumConfig] + labels[LabelKeyChecksumSigningCA]; len(infix) > 0 {
			name += "-" + utils.ComputeSHA256Hex([]byte(infix))[:8]
		}
	}

	if suffix := labels[LabelKeyLastRotationStartedTime]; len(suffix) > 0 {
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
		Immutable:  pointer.Bool(true),
	}
}

func secretTypeForData(data map[string][]byte) corev1.SecretType {
	secretType := corev1.SecretTypeOpaque
	if data[secretutils.DataKeyCertificate] != nil && data[secretutils.DataKeyPrivateKey] != nil {
		secretType = corev1.SecretTypeTLS
	}
	return secretType
}
