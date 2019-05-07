package encryptionconfiguration

// Note that kind and apiversion need to match the EncryptionConfiguration
const (
	ecKind                               string = "EncryptionConfiguration"
	ecAPIVersion                         string = "apiserver.config.k8s.io/v1"
	ecKeyPrefix                          string = "key"
	ecResouceConfigurationLenE           int    = 1
	ecEncryptedResourceSecrets           string = "secrets"
	ecEncryptionProviderAESCBC           string = "aescbc"
	ecEncryptionProviderIdentity         string = "identity"
	ecEncryptionProviderLenE             int    = 2
	ecEncryptionProviderAESCBCMinKeyLenE int    = 1
)

var ecEncryptedResources = []string{
	ecEncryptedResourceSecrets,
}
