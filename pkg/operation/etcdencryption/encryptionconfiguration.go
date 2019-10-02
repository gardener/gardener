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

package encryptionconfiguration

import (
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/operation/common"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

var (
	codec runtime.Codec
)

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(apiserverconfigv1.AddToScheme(scheme))
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)
	codec = versioning.NewDefaultingCodecForScheme(
		scheme,
		serializer,
		serializer,
		apiserverconfigv1.SchemeGroupVersion,
		apiserverconfigv1.SchemeGroupVersion)
}

// NewEncryptionKey creates a new random encryption key with a name containing the timestamp.
// The reader should return random data suitable for cryptographic use, otherwise the security
// of encryption might be compromised.
func NewEncryptionKey(t time.Time, r io.Reader) (*apiserverconfigv1.Key, error) {
	keyName := NewEncryptionKeyName(t)
	keySecret, err := NewEncryptionKeySecret(r)
	if err != nil {
		return nil, err
	}

	return &apiserverconfigv1.Key{
		Name:   keyName,
		Secret: keySecret,
	}, nil
}

// NewEncryptionKeyName creates a new key with the given timestamp.
func NewEncryptionKeyName(t time.Time) string {
	return fmt.Sprintf("%s%d", common.EtcdEncryptionKeyPrefix, t.Unix())
}

// ParseEncryptionKeyName parses the key name.
func ParseEncryptionKeyName(keyName string) (time.Time, error) {
	if strings.HasPrefix(common.EtcdEncryptionKeyPrefix, keyName) {
		return time.Time{}, fmt.Errorf("key does not start with prefix %s", common.EtcdEncryptionKeyPrefix)
	}

	n, err := strconv.ParseInt(strings.TrimPrefix(keyName, common.EtcdEncryptionKeyPrefix), 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(n, 0), nil
}

// NewEncryptionKeySecret reads common.EtcdEncryptionSecretLen bytes from the given reader
// and base-64 encodes the data.
// The reader should return random data suitable for cryptographic use, otherwise the security
// of encryption might be compromised.
func NewEncryptionKeySecret(r io.Reader) (string, error) {
	buf := make([]byte, common.EtcdEncryptionKeySecretLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("could not read enough data: %v", err)
	}

	sEnc := base64.StdEncoding.EncodeToString(buf)
	return sEnc, nil
}

// NewPassiveConfiguration creates an initial configuration for etcd encryption
// The list of encryption providers contains identity as first provider, which has
// the effect, that this configuration does not yet encrypt written secrets. The
// configuration has to be activated to actually encrypt written secrets.
// Nevertheless, an encryption provider aescbc is already contained in the configuration
// at the second position in the list of providers. A key is created for aescbc with
// the key's name containing the given time.
//
// apiVersion: apiserver.config.k8s.io/v1
// kind: EncryptionConfiguration
// resources:
// - providers:
//   - identity: {}
//   - aescbc:
//       keys:
//       - name: key1559747207815249000
//         secret: Y8LEzbtK/2mdXrw6W/faAxNLu+mTCmcQeWojShAJGEg=
//   resources:
//     metadata:
//   - secrets
func NewPassiveConfiguration(t time.Time, r io.Reader) (*apiserverconfigv1.EncryptionConfiguration, error) {
	key, err := NewEncryptionKey(t, r)
	if err != nil {
		return nil, err
	}

	return &apiserverconfigv1.EncryptionConfiguration{
		Resources: []apiserverconfigv1.ResourceConfiguration{
			{
				Resources: []string{common.EtcdEncryptionEncryptedResourceSecrets},
				Providers: []apiserverconfigv1.ProviderConfiguration{
					{Identity: &apiserverconfigv1.IdentityConfiguration{}},
					{AESCBC: &apiserverconfigv1.AESConfiguration{
						Keys: []apiserverconfigv1.Key{
							*key,
						},
					}},
				},
			},
		},
	}, nil
}

// Load decodes an EncryptionConfiguration from the given data.
func Load(data []byte) (*apiserverconfigv1.EncryptionConfiguration, error) {
	ec := &apiserverconfigv1.EncryptionConfiguration{}
	if _, _, err := codec.Decode(data, nil, ec); err != nil {
		return nil, err
	}

	return ec, nil
}

// Write encodes an EncryptionConfiguration.
func Write(ec *apiserverconfigv1.EncryptionConfiguration) ([]byte, error) {
	return runtime.Encode(codec, ec)
}

func isEncryptingProviderConfiguration(conf *apiserverconfigv1.ProviderConfiguration) bool {
	return conf.Identity == nil
}

func findResourceConfigurationForResource(configs []apiserverconfigv1.ResourceConfiguration, resource string) (*apiserverconfigv1.ResourceConfiguration, error) {
	for _, config := range configs {
		for _, r := range config.Resources {
			if r == resource {
				return &config, nil
			}
		}
	}
	return nil, fmt.Errorf("no resource configuration found for resource %q", resource)
}

// SetResourceEncryption sets the EncryptionConfiguration to active or non-active (passive) state.
// State active means that provider aescbc is the first in the list of providers.
// State non-active (passive) means that provider identity is the first in the list of providers.
func SetResourceEncryption(c *apiserverconfigv1.EncryptionConfiguration, resource string, encrypted bool) error {
	conf, err := findResourceConfigurationForResource(c.Resources, resource)
	if err != nil {
		return err
	}

	for i := 0; i < len(conf.Providers); i++ {
		if isEncryptingProviderConfiguration(&conf.Providers[i]) == encrypted {
			if i == 0 {
				return nil
			}

			tmp := conf.Providers[0]
			conf.Providers[0] = conf.Providers[i]
			conf.Providers[i] = tmp
			return nil
		}
	}
	return fmt.Errorf("no encryption provider configuration found for to set encryption of resource %q to %t", resource, encrypted)
}

var errConfigurationNotFound = fmt.Errorf("no encryption configuration at %s", common.EtcdEncryptionSecretFileName)

// IsConfigurationNotFoundError checks if the given error is an error when the encryption
// configuration is not found at the common.EtcdEncryptionSecretFileName key of the data section
// of a secret.
func IsConfigurationNotFoundError(err error) bool {
	return err == errConfigurationNotFound
}

// ReadSecret reads and validates the EncryptionConfiguration of the given secret.
func ReadSecret(secret *corev1.Secret) (*apiserverconfigv1.EncryptionConfiguration, error) {
	confData, ok := secret.Data[common.EtcdEncryptionSecretFileName]
	if !ok {
		return nil, errConfigurationNotFound
	}

	conf, err := Load(confData)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

// UpdateSecret writes the EncryptionConfiguration to the common.EtcdEncryptionSecretFileName key
// in the data section of the given secret.
func UpdateSecret(secret *corev1.Secret, conf *apiserverconfigv1.EncryptionConfiguration) error {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	confData, err := Write(conf)
	if err != nil {
		return err
	}

	secret.Data[common.EtcdEncryptionSecretFileName] = confData
	return nil
}
