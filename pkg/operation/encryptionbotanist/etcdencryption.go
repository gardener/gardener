package encryptionbotanist

import "github.com/gardener/gardener/pkg/logger"

type encryptionBotanistImpl struct{}

func (e *encryptionBotanistImpl) StartEtcdEncryption() error {
	logger.Logger.Info("Starting Etcd Encryption")

	return nil
}

func New() (EncryptionBotanist, error) {
	return &encryptionBotanistImpl{}, nil
}
