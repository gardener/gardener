package encryptionbotanist

type EncryptionBotanist interface {
	StartEtcdEncryption() error
}

const (
	// EtcdEncryptionSecret is a constant for the name of the secret which contains the etcd encryption key
	EtcdEncryptionSecret = "etcd-encryption-secret"
)
