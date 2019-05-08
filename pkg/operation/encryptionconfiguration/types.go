package encryptionconfiguration

// Note that kind and apiversion need to match the EncryptionConfiguration
const (
	ecKind                               string = "EncryptionConfiguration"
	ecAPIVersion                         string = "apiserver.config.k8s.io/v1"
	ecKeyPrefix                          string = "key"
	ecKeySecretLen                       int    = 32
	ecResouceConfigurationLenE           int    = 1
	ecEncryptedResourceSecrets           string = "secrets"
	ecEncryptionProviderAESCBC           string = "aescbc"
	ecEncryptionProviderIdentity         string = "identity"
	ecEncryptionProviderLenE             int    = 2
	ecEncryptionProviderAESCBCMinKeyLenE int    = 1
	ecKeyTimestampMaxClockSkewNanos      int64  = 24 * 60 * 60 * 1000000000
	ecKeyTimestampMaxAgeNanos            int64  = 1557323541869370000
)

var ecEncryptedResources = []string{
	ecEncryptedResourceSecrets,
}
