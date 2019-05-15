package encryptionconfiguration

import (
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

// CreateNewPassiveConfiguration creates a initial configuration for etcd encryption
// The list of encryption providers contains identity as first provider, which has
// the effect, that this configuration does not yet encrypt written secrets. The
// configuration has to be activated to actually encrypt written secrets.
// Nevertheless, an encryption provider aescbc is already contained in the configuration
// at the second position in the list of providers. A key is created for aescbc with
// the key's name containing the timestamp when it was created in UTC.
//
//
// kind: EncryptionConfiguration
// apiVersion: apiserver.config.k8s.io/v1
// resources:
//   - resources:
//     - secrets
//     providers:
//     - aescbc:
//         keys:
//         - name: key1553679720
//           secret: t44dGAwGt73RMOSNwp4Z9QXadtnLvC4fZWgzS8Tjz+c=
//     - identity: {}
func CreateNewPassiveConfiguration() (*apiserverconfigv1.EncryptionConfiguration, error) {
	key, err := createEncryptionKey()
	if err != nil {
		return nil, err
	}
	ec := apiserverconfigv1.EncryptionConfiguration{
		TypeMeta: v1.TypeMeta{
			Kind:       ecKind,
			APIVersion: ecAPIVersion,
		},
		Resources: []apiserverconfigv1.ResourceConfiguration{
			{
				Resources: ecEncryptedResources,
				Providers: []apiserverconfigv1.ProviderConfiguration{
					{Identity: &apiserverconfigv1.IdentityConfiguration{}},
					{AESCBC: &apiserverconfigv1.AESConfiguration{
						Keys: []apiserverconfigv1.Key{
							key,
						},
					}},
				},
			},
		},
	}
	return &ec, nil
}

// CreateFromYAML creates a new configuration from a YAML String as Byte Array.
func CreateFromYAML(ecYamlBytes []byte) (*apiserverconfigv1.EncryptionConfiguration, error) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	utilruntime.Must(apiserverconfigv1.AddToScheme(scheme))
	ec := &apiserverconfigv1.EncryptionConfiguration{}
	_, _, err := codecs.UniversalDecoder().Decode(ecYamlBytes, nil, ec)
	if err != nil {
		return nil, fmt.Errorf("error while decoding EncryptionConfiguration from yamlArray: %v", err)
	}
	return ec, nil
}

// ToYAML Creates a YAML representation of the EncryptionConfiguration.
func ToYAML(ec *apiserverconfigv1.EncryptionConfiguration) ([]byte, error) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	err := apiserverconfigv1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("error while preparing parsing of EncryptionConfiguration: %v", err)
	}
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)
	encoder := codecs.EncoderForVersion(serializer, apiserverconfigv1.SchemeGroupVersion)
	ecYamlBytes, err := runtime.Encode(encoder, ec)
	if err != nil {
		return nil, fmt.Errorf("error while parsing EncryptionConfiguration: %v", err)
	}
	return ecYamlBytes, nil
}

// IsConsistent checks whether the configuration is consistent.
// Consistency checks include the following:
// Has the secret etcd-encryption-secret a data item named encryption-configuration.yaml?
// Consists the list of encryption providers of exactly the following 2 entries: "identity", "aescbc")?
// Is a key present with a sensible timestamp in its name and a sensible value?
// Is the additional data item encryption-metadata.yaml available and valid?
// Is the secret made available as a volume and via a volume mount to the API server pod in the shoot's API server deployment?
// Is the file encryption-configuration.yaml referenced in a startup parameter --encryption-provider-config of the shoot's API server?
func IsConsistent(ec *apiserverconfigv1.EncryptionConfiguration) (bool, error) {
	if ec.Kind != ecKind {
		return false, fmt.Errorf("kind (%v) of the EncryptionConfiguration does not match the expected kind (%v)", ec.Kind, ecKind)
	}
	if ec.APIVersion != ecAPIVersion {
		return false, fmt.Errorf("apiversion (%v) of the EncryptionConfiguration does not match the expected apiversion (%v)", ec.APIVersion, ecAPIVersion)
	}
	ecResouceConfigurationLenA := len(ec.Resources)
	if ecResouceConfigurationLenA != ecResouceConfigurationLenE {
		return false, fmt.Errorf("number of resource configurations (%v) of the EncryptionConfiguration does not match the expected number (%v)", ecResouceConfigurationLenA, ecResouceConfigurationLenE)
	}
	// Check resources
	if !slicesContainSameElements(ec.Resources[0].Resources, ecEncryptedResources) {
		return false, fmt.Errorf("list of encrypted resources (%v) of the EncryptionConfiguration does not match the expected list (%v)", ec.Resources[0].Resources, ecEncryptedResources)
	}
	// Check encryption providers
	ecEncryptionProviderLenA := len(ec.Resources[0].Providers)
	if ecEncryptionProviderLenA != ecEncryptionProviderLenE {
		return false, fmt.Errorf("number of encryption providers (%v) of the EncryptionConfiguration does not match the expected number (%v)", ecEncryptionProviderLenA, ecEncryptionProviderLenE)
	}
	// Encryption provider aescbc and identity in any order are ok
	if !(((ec.Resources[0].Providers[0].Identity != nil) && (ec.Resources[0].Providers[1].AESCBC != nil)) ||
		((ec.Resources[0].Providers[0].AESCBC != nil) && (ec.Resources[0].Providers[1].Identity != nil))) {
		return false, fmt.Errorf("unexpected encryption providers of the EncryptionConfiguration found. Expected are exactly two providers 'identity' and 'aescbc'")
	}
	var aesConfig *apiserverconfigv1.AESConfiguration
	if ec.Resources[0].Providers[0].AESCBC != nil {
		aesConfig = ec.Resources[0].Providers[0].AESCBC
	} else {
		aesConfig = ec.Resources[0].Providers[1].AESCBC
	}
	ecEncryptionProviderAESCBCKeyLenA := len(aesConfig.Keys)
	if ecEncryptionProviderAESCBCKeyLenA < ecEncryptionProviderAESCBCMinKeyLenE {
		return false, fmt.Errorf("unexpected number of keys in encryption provider aescbc of the EncryptionConfiguration found. Expected are at least %v key(s)", ecEncryptionProviderAESCBCMinKeyLenE)
	}
	keyNameMap := make(map[string]bool)
	for _, key := range aesConfig.Keys {
		ok, err := isKeyConsistent(key)
		if (err != nil) || (!ok) {
			return false, fmt.Errorf("inconsistent key in encryption provider aescbc of the EncryptionConfiguration found")
		}
		_, ok = keyNameMap[key.Name]
		if ok {
			return false, fmt.Errorf("two or more keys (%v) with same timestamp found in encryption provider aescbc of the EncryptionConfiguration", key.Name)
		} else {
			keyNameMap[key.Name] = true
		}
	}
	return true, nil
}

// Equals checks whether the provided encryption configurations are equal.
func Equals(ec1 *apiserverconfigv1.EncryptionConfiguration, ec2 *apiserverconfigv1.EncryptionConfiguration) (bool, error) {
	ec1YamlBytes, err := ToYAML(ec1)
	if err != nil {
		return false, fmt.Errorf("error when converting EncryptionConfiguration to yaml: %v", err)
	}
	ec1YamlString := string(ec1YamlBytes)
	ec2YamlBytes, err := ToYAML(ec2)
	if err != nil {
		return false, fmt.Errorf("error when converting EncryptionConfiguration to yaml: %v", err)
	}
	ec2YamlString := string(ec2YamlBytes)
	if ec1YamlString == ec2YamlString {
		return true, nil
	} else {
		return false, nil
	}
}

// IsActive checks whether the EncryptionConfiguration is active, i.e. whether the provider
// identity is NOT the first in the list of providers.
func IsActive(ec *apiserverconfigv1.EncryptionConfiguration) bool {
	if ec.Resources[0].Providers[0].AESCBC != nil {
		return true
	} else {
		return false
	}
}

// SetActive sets the EncryptionConfiguration to active or non-active (passive) state.
// State active means that provider aescbc is the first in the list of providers.
// State non-active (passive) means that provider identity is the first in the list of providers.
func SetActive(ec *apiserverconfigv1.EncryptionConfiguration, active bool) error {
	a := IsActive(ec)
	if a == active {
		// nothing to do
		return nil
	}
	pc := ec.Resources[0].Providers[0]
	ec.Resources[0].Providers[0] = ec.Resources[0].Providers[1]
	ec.Resources[0].Providers[1] = pc
	return nil
}

// CreatePassiveRotationKey adds a new key to the EncryptionConfiguration as a second key
// in the list of keys. Note that the order matters and the first key in the list
// of keys is used for encrypting etcd contents.
func CreatePassiveRotationKey(ec *apiserverconfigv1.EncryptionConfiguration) error {
	return fmt.Errorf("not implemented yet")
}

// IsRotationKeyActive checks whether the most current key (i.e. the rotation key) is
// also the first key in the list of keys.
func IsRotationKeyActive(ec *apiserverconfigv1.EncryptionConfiguration) (bool, error) {
	return false, fmt.Errorf("not implemented yet")
}

// ActivateRotationKey ensures that the newest key is also the first key in the
// list of keys.
func ActivateRotationKey(ec *apiserverconfigv1.EncryptionConfiguration) error {
	return fmt.Errorf("not implemented yet")
}

// PruneOldEncryptionKey removes the old key from the list of keys.
func PruneOldEncryptionKey(ec *apiserverconfigv1.EncryptionConfiguration) error {
	return fmt.Errorf("not implemented yet")
}
