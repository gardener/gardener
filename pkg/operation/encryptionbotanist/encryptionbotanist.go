package encryptionbotanist

import (
	"fmt"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/encryptionconfiguration"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

type encryptionBotanistImpl struct {
	*operation.Operation
}

func (e *encryptionBotanistImpl) StartEtcdEncryption() error {
	logger.Logger.Info("Starting Etcd Encryption")

	// TODO: question do we need a switch to de-activate the encryption feature (although we all agreed to have 'secure by default')?

	exists, ec, err := e.readEncryptionConfigurationFromSeed()
	if err != nil {
		return err
	}
	if !exists {
		ec, err = encryptionconfiguration.CreateNewPassiveConfiguration()
		if err != nil {
			return err
		}
		err = e.writeConfiguration(ec)
		if err != nil {
			return err
		}
		err = e.enableConfiguration()
		if err != nil {
			return err
		}
	}

	consistent, err := e.isConfigurationConsistent()
	if (err != nil) || !consistent {
		return err
	}

	enabled, err := e.isConfigurationEnabled()
	if (err != nil) || !enabled {
		return err
	}

	// WARNING:
	// No explicit checking of whether EncryptionConfiguration is contained in a backup.
	// Be aware of the risk!
	//
	// TODO: Ensure this is also agreed upon by Gardener team
	if !encryptionconfiguration.IsActive(ec) {
		encryptionconfiguration.SetActive(ec)
		err = e.writeConfiguration(ec)
		if err != nil {
			return err
		}
	}

	needToRewrite, err := e.needToRewriteShootSecrets()
	if err != nil {
		return err
	}
	if needToRewrite {
		err = e.rewriteShootSecrets()
		if err != nil {
			return err
		}
		err = e.setNeedToRewriteShootSecrets(false)
		if err != nil {
			return err
		}
	}

	return nil
}

// New creates a new EncryptionBotanist
func New(o *operation.Operation) (EncryptionBotanist, error) {
	return &encryptionBotanistImpl{
		Operation: o,
	}, nil
}

// readEncryptionConfigurationFromSeed reads the EncryptionConfiguration from the shoot namespace in the seed
func (e *encryptionBotanistImpl) readEncryptionConfigurationFromSeed() (bool, *apiserverconfigv1.EncryptionConfiguration, error) {
	// ****************************************************************************************************************
	// TODO: Check Pseudocode
	//
	// 1. obtain e.Operation.K8sSeedClient
	// 2. switch to shoot namespace
	// 3. read etcd-encryption-secret
	// ec, err := encryptionconfiguration.CreateFromYAML(secretData)
	// if err != nil {
	// 	return false, nil, fmt.Errorf("EncryptionConfiguration in seed cluster is not consistent: %v", err)
	// }
	// ****************************************************************************************************************

	return false, nil, nil
}

// readEncryptionConfigurationFromGarden reads the EncryptionConfiguration from the shoot namespace in the seed
func (e *encryptionBotanistImpl) readEncryptionConfigurationFromGarden() (bool, *apiserverconfigv1.EncryptionConfiguration, error) {
	// ****************************************************************************************************************
	// TODO: Check Pseudocode
	//
	// 1. obtain e.Operation.K8sGardenClient
	// 2. switch to shoot namespace
	// 3. read etcd-encryption-secret
	//
	// ec, err := encryptionconfiguration.CreateFromYAML(secretData)
	// if err != nil {
	// 	return false, nil, fmt.Errorf("EncryptionConfiguration in seed cluster is not consistent: %v", err)
	// }
	// ****************************************************************************************************************
	return false, nil, nil
}

// writeConfiguration writes the secret which contains the EncryptionConfiguration to the
// shoot namespace in the seed cluster as well as to the garden cluster
func (e *encryptionBotanistImpl) writeConfiguration(ec *apiserverconfigv1.EncryptionConfiguration) error {
	err := e.writeConfigurationSecretToSeed(ec)
	if err != nil {
		return err
	}
	err = e.writeConfigurationSecretToGarden(ec)
	if err != nil {
		return err
	}
	return nil
}

// writeConfigurationSecretToSeed writes the secret which contains the EncryptionConfiguration
// to the shoot namespace in the seed cluster
func (e *encryptionBotanistImpl) writeConfigurationSecretToSeed(ec *apiserverconfigv1.EncryptionConfiguration) error {
	// ****************************************************************************************************************
	// TODO: Check Pseudocode
	//
	// 1. obtain e.Operation.K8sSeedClient
	// 2. switch to shoot namespace
	// 3. write etcd-encryption-secret
	//
	// TODO questions:
	// - will this automatically restart the apiserver(s) since the secret is (once enabled) mounted to the apiserver
	// ****************************************************************************************************************
	return nil
}

// writeConfigurationSecretToSeed writes the secret which contains the EncryptionConfiguration
// to the garden cluster
func (e *encryptionBotanistImpl) writeConfigurationSecretToGarden(ec *apiserverconfigv1.EncryptionConfiguration) error {
	// ****************************************************************************************************************
	// TODO: Check Pseudocode
	//
	// 1. obtain e.Operation.K8sGardenClient
	// 2. switch to shoot namespace
	// 3. write etcd-encryption-secret
	// ****************************************************************************************************************
	return nil
}

// enableConfiguration enables the configuration and thus modifies the shoot's apiserver deployment in the seed cluster
// - creates a volume for the EncryptionConfiguration
// - creates a volume mount
// - sets the apiserver start parameter
func (e *encryptionBotanistImpl) enableConfiguration() error {
	// ****************************************************************************************************************
	// TODO: Check Pseudocode
	//
	// 1. obtain e.Operation.K8sSeedClient
	// 2. switch to shoot namespace
	// 3. read deployment of apiserver
	// 4. adapt deployment (urks, not easy to change the params)
	// 5. write deployment
	//
	// TODO questions:
	// - will this automatically restart the apiserver(s) ?
	// - how will processing of this reconsiliation continue then?
	// ****************************************************************************************************************

	return nil
}

// isConfigurationEnabled checks whether the following parameters exist in the shoot's
// apiserver deployment as expected
// - volume for the EncryptionConfiguration
// - volume mount
// - apiserver start parameter
func (e *encryptionBotanistImpl) isConfigurationEnabled() (bool, error) {
	// ****************************************************************************************************************
	// TODO: Check Pseudocode
	//
	// 1. obtain e.Operation.K8sSeedClient
	// 2. switch to shoot namespace
	// 3. read deployment of apiserver
	// 4. verify deployment (urks, not easy to parse the params)
	//
	// ****************************************************************************************************************
	return false, nil
}

// isConfigurationConsistent checks whether the configuration is consistent and enabled
func (e *encryptionBotanistImpl) isConfigurationConsistent() (bool, error) {
	exists, ecSeed, err := e.readEncryptionConfigurationFromSeed()
	if (err != nil) || !exists {
		return false, err
	}
	exists, ecGarden, err := e.readEncryptionConfigurationFromGarden()
	if (err != nil) || !exists {
		return false, err
	}
	equal, err := encryptionconfiguration.Equals(ecSeed, ecGarden)
	if (err != nil) || !equal {
		return false, fmt.Errorf("EncryptionConfiguration in seed cluster and garden cluster are not equal: %v", err)
	}
	consistent, err := encryptionconfiguration.IsConsistent(ecSeed)
	if (err != nil) || !consistent {
		return false, fmt.Errorf("EncryptionConfiguration in seed cluster is not consistent: %v", err)
	}
	return true, nil
}

// needToRewriteShootSecrets checks whether the secrets in the shoot need to
// be rewritten, e.g. after a change to the EncryptionConfiguration
func (e *encryptionBotanistImpl) needToRewriteShootSecrets() (bool, error) {
	// ****************************************************************************************************************
	// TODO: Check Pseudocode
	//
	// 1. obtain e.Operation.K8sSeedClient
	// 2. switch to shoot namespace
	// 3. check annotation (how?)
	//
	// ****************************************************************************************************************
	return false, nil
}

// setNeedToRewriteShootSecrets sets the annotation with which to remember
// whether the shoot secrets need to be rewritten
func (e *encryptionBotanistImpl) setNeedToRewriteShootSecrets(rewrite bool) error {
	// ****************************************************************************************************************
	// TODO: Check Pseudocode
	//
	// 1. obtain e.Operation.K8sSeedClient
	// 2. switch to shoot namespace
	// 3. set annotation (how?)
	//
	// ****************************************************************************************************************
	return nil
}

// rewriteShootSecrets rewrites all secrets of the shoot.
// This will take into account the current EncryptionConfiguration.
func (e *encryptionBotanistImpl) rewriteShootSecrets() error {
	// ****************************************************************************************************************
	// TODO: Check Pseudocode
	//
	// 1. obtain e.Operation.K8sShootClient
	// 2. For all secrets in all namespaces:
	//    a) read secret
	//    b) write secret
	//
	// ****************************************************************************************************************
	return nil
}

func (e *encryptionBotanistImpl) testAndPlay() {
	// pl, err := e.Operation.K8sSeedClient.ListPods(e.Operation.Shoot.SeedNamespace, metav1.ListOptions{})
	// if err != nil {
	// 	return err
	// }
	// for _, pod := range pl.Items {
	// 	if pod.DeletionTimestamp != nil {
	// 		continue
	// 	}
	// 	name := pod.GetName()
	// 	logger.Logger.Info("pod: " + name)
	// }

	//deployment, err := e.Operation.K8sSeedClient.GetDeployment(e.Operation.Shoot.SeedNamespace, common.KubeAPIServerDeploymentName)
	//logger.Logger.Info("deployment command length: ", len(deployment.Spec.Template.Spec.Containers[0].Command))
	//logger.Logger.Info("deployment: ", deployment.Spec.Template.Spec.Containers[0].Command[0])
	//logger.Logger.Info("deployment: ", deployment.Spec.Template.Spec.Containers[0].Command[1])
	//var manifest = []byte("")
	//e.Operation.K8sSeedClient.Applier().ApplyManifest(context.TODO(), kubernetes.NewManifestReader(manifest), kubernetes.DefaultApplierOptions)
	//e.Operation.
}
