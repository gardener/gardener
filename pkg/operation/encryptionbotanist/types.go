package encryptionbotanist

// EncryptionBotanist encapsules the configuration of etcd encryption
type EncryptionBotanist interface {

	// StartEtcdEncryption triggers the configuration process
	StartEtcdEncryption() error
}

const (
	// EtcdEncryptionSecret is a constant for the name of the secret which contains the etcd encryption key
	EtcdEncryptionSecret = "etcd-encryption-secret"
)
