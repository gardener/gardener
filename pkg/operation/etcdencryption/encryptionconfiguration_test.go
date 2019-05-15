package encryptionconfiguration

import (
	"testing"

	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

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
func TestCreateNewPassiveConfiguration(t *testing.T) {
	_, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
}

func TestCreateToYAMLFromYAML(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ecYamlBytes, err := ToYAML(ec)
	if err != nil {
		t.Fatalf("error during YAML creation: %v", err)
	}
	t.Log(string(ecYamlBytes))
	ec2, err := CreateFromYAML(ecYamlBytes)
	if err != nil {
		t.Fatalf("error during object creation from YAML string: %v", err)
	}
	str2, err := ToYAML(ec2)
	if err != nil {
		t.Fatalf("error during second YAML creation: %v", err)
	}
	t.Log(string(str2))
}

func TestConsistencyCorrect(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ok, err := IsConsistent(ec)
	if (err != nil) || (!ok) {
		t.Fatalf("newly generated EncryptionConfiguration ought to be consistent: %v", err)
	}
}

func TestConsistencyIncorrectWrongKind(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.Kind = "wrong"
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (wrong kind) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrectWrongAPIVersion(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.APIVersion = "v1"
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (wrong APIVersion) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrectNoResourceConfiguration(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.Resources = nil
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (no resource configuration) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrectTooManyResourceConfigurations(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.Resources = []apiserverconfigv1.ResourceConfiguration{
		{},
		{},
	}
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (no resource configuration) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrectUnsupportedResource(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.Resources[0].Resources = []string{
		"unknownresource",
	}
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (unsupported resource to be encrypted) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrectNoProviders(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.Resources[0].Providers = nil
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (no encryption providers) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrectTooManyProviders(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.Resources[0].Providers = []apiserverconfigv1.ProviderConfiguration{
		{},
		{},
		{},
	}
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (too many encryption providers) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrectJustProviderIdentity(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.Resources[0].Providers = []apiserverconfigv1.ProviderConfiguration{
		{Identity: &apiserverconfigv1.IdentityConfiguration{}},
		{Identity: &apiserverconfigv1.IdentityConfiguration{}},
	}
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (twice encryption provider identity) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrectProvidersIdentityAndAESGCM(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.Resources[0].Providers = []apiserverconfigv1.ProviderConfiguration{
		{Identity: &apiserverconfigv1.IdentityConfiguration{}},
		{AESGCM: &apiserverconfigv1.AESConfiguration{
			Keys: []apiserverconfigv1.Key{},
		}}}
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (identity and aesgcm) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrectNoKeysInAESCBC(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ec.Resources[0].Providers = []apiserverconfigv1.ProviderConfiguration{
		{Identity: &apiserverconfigv1.IdentityConfiguration{}},
		{AESCBC: &apiserverconfigv1.AESConfiguration{
			Keys: []apiserverconfigv1.Key{},
		}}}
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (no keys in configuration aescbc) of EncryptionConfiguration not detected")
	}
}

func TestConsistencyIncorrecKeysWithSameNameInAESCBC(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	key1, _ := createEncryptionKey()
	key2, _ := createEncryptionKey()
	key3, _ := createEncryptionKey()
	key4, _ := createEncryptionKey()
	key4.Name = key2.Name
	key5, _ := createEncryptionKey()
	ec.Resources[0].Providers = []apiserverconfigv1.ProviderConfiguration{
		{Identity: &apiserverconfigv1.IdentityConfiguration{}},
		{AESCBC: &apiserverconfigv1.AESConfiguration{
			Keys: []apiserverconfigv1.Key{
				key1,
				key2,
				key3,
				key4,
				key5,
			},
		}}}
	ok, err := IsConsistent(ec)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (two keys with same name) of EncryptionConfiguration not detected")
	}
}

func TestPassive(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	active := IsActive(ec)
	if err != nil {
		t.Fatalf("error during IsActive: %v", err)
	}
	if active {
		t.Fatalf("a passive EncryptionConfiguration should be in fact passive")
	}
}

func TestSetActive(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ecYamlBytes, err := ToYAML(ec)
	t.Log(string(ecYamlBytes))
	err = SetActive(ec, true)
	if err != nil {
		t.Fatalf("error during SetActive: %v", err)
	}
	ecYamlBytes, err = ToYAML(ec)
	t.Log(string(ecYamlBytes))
	active := IsActive(ec)
	if !active {
		t.Fatalf("the activated EncryptionConfiguration is not active")
	}
}

const (
	passiveConfig = `
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - identity: {}
  - aescbc:
      keys:
      - name: key1557839139300191000
        secret: kL03D325eDntRBfLtlh0jFSqQE0Ji69bVq421IwJGiI=
  resources:
  - secrets`
	activeConfig = `
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:
      - name: key1557839139300191000
        secret: kL03D325eDntRBfLtlh0jFSqQE0Ji69bVq421IwJGiI=
  - identity: {}
  resources:
  - secrets
`
)

func TestSetActiveWithYAML(t *testing.T) {
	ecActive, err := CreateFromYAML([]byte(activeConfig))
	if err != nil {
		t.Fatalf("error during object creation from YAML string: %v", err)
	}
	ecActiveYamlBytes, err := ToYAML(ecActive)
	if err != nil {
		t.Fatalf("error during second YAML creation: %v", err)
	}
	ecActiveYamlString := string(ecActiveYamlBytes)
	ecPassive, err := CreateFromYAML([]byte(passiveConfig))
	if err != nil {
		t.Fatalf("error during object creation from YAML string: %v", err)
	}
	SetActive(ecPassive, true)
	ecPassiveYamlBytes, err := ToYAML(ecPassive)
	if err != nil {
		t.Fatalf("error during second YAML creation: %v", err)
	}
	ecPassiveYamlString := string(ecPassiveYamlBytes)
	if ecActiveYamlString != ecPassiveYamlString {
		t.Fatalf("unexpected result of activation of encryption configuration.\nExpected:\n%v\n\nActual:\n%v", ecActiveYamlString, ecPassiveYamlString)
	}
}
