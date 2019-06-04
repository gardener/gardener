package botanist

import (
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	encryptionconfiguration "github.com/gardener/gardener/pkg/operation/etcdencryption"
	"github.com/gardener/gardener/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/util/retry"
)

// CreateEtcdEncryptionConfiguration creates a secret
func (b *Botanist) CreateEtcdEncryptionConfiguration() error {
	b.Logger.Info("starting CreateEtcdEncryptionConfiguration")

	enableEtcdEncryption, err := utils.CheckVersionMeetsConstraint(b.Shoot.Info.Spec.Kubernetes.Version, ">= 1.13")
	if err != nil {
		return err
	}
	if !enableEtcdEncryption {
		b.Logger.Info("support for etcd encryption is offered automatically starting with kubernetes version 1.13")
		return nil
	}
	needToWriteConfig := false
	exists, ecSeed, forcePlaintext, err := b.readEncryptionConfigurationFromSeed()
	if err != nil {
		return err
	}
	if !exists {
		// create new passive EncryptionConfiguration if it does not exist yet
		// and remember to write this created configuration
		b.Logger.Info("no EncryptionConfiguration found in seed. Creating new passive configuration.")
		ecSeed, err = encryptionconfiguration.CreateNewPassiveConfiguration()
		if err != nil {
			return err
		}
		needToWriteConfig = true
	} else {
		exists, ecGarden, err := b.readEncryptionConfigurationFromGarden()
		if err != nil {
			return err
		} else if !exists {
			return fmt.Errorf("EncryptionConfiguration exists in seed but not in garden")
		}
		// if it exists already, it needs to be consistent
		consistent, err := b.isEncryptionConfigurationConsistent(ecSeed, ecGarden)
		if (err != nil) || !consistent {
			return err
		}
		if forcePlaintext {
			// if annotation requires plaintext secrets, thou shalt get plaintext secrets
			if encryptionconfiguration.IsActive(ecSeed) {
				// deactivate configuration if it is still active
				b.Logger.Info("de-activating existing active EncryptionConfiguration. Note: shoot secrets will be decrypted in etcd subsequently")
				encryptionconfiguration.SetActive(ecSeed, false)
				needToWriteConfig = true
			}
		} else if !encryptionconfiguration.IsActive(ecSeed) {
			// if it is not active already (aescbc as first provider) then set it to active
			// and remember to write this created configuration
			b.Logger.Info("activating existing passive EncryptionConfiguration")
			encryptionconfiguration.SetActive(ecSeed, true)
			needToWriteConfig = true
		}
	}
	ecSeedYamlBytes, err := encryptionconfiguration.ToYAML(ecSeed)
	if err != nil {
		return err
	}
	if needToWriteConfig {
		b.Logger.Info("writing new/updated EncryptionConfiguration")
		err = b.writeEncryptionConfiguration(ecSeedYamlBytes)
		if err != nil {
			return err
		}
	}

	checksum := utils.ComputeSHA256Hex(ecSeedYamlBytes)
	b.mutex.Lock()
	b.CheckSums[common.EtcdEncryptionSecretName] = checksum
	b.mutex.Unlock()
	// enablement of etcd encryption feature done in helm chart of apiserver deployment
	return nil
}

// RewriteShootSecretsIfEncryptionConfigurationChanged rewrites a shoot's secrets if the EncryptionConfiguration has changed
func (b *Botanist) RewriteShootSecretsIfEncryptionConfigurationChanged() error {
	b.Logger.Info("starting RewriteShootSecretsIfEncryptionConfigurationChanged")
	// WARNING:
	// No explicit checking of whether EncryptionConfiguration is contained in a backup.
	// Be aware of the risk!
	//
	// TODO: Ensure this is also agreed upon by Gardener team

	// TODO: contact Amshuman Rao Karaya

	enableEtcdEncryption, err := utils.CheckVersionMeetsConstraint(b.Shoot.Info.Spec.Kubernetes.Version, ">= 1.13")
	if err != nil {
		return err
	}
	if !enableEtcdEncryption {
		return nil
	}
	needToRewrite, err := b.needToRewriteShootSecrets()
	if err != nil {
		return err
	}
	if needToRewrite {
		b.Logger.Info("rewriting shoot secrets")
		err = b.rewriteShootSecrets()
		if err != nil {
			return err
		}
	} else {
		b.Logger.Trace("EncryptionConfiguration unchanged, no rewriting necessary")
	}
	return nil
}

// readEncryptionConfigurationFromSeed reads the EncryptionConfiguration from the shoot namespace in the seed
func (b *Botanist) readEncryptionConfigurationFromSeed() (exists bool, ec *apiserverconfigv1.EncryptionConfiguration, forcePlaintext bool, err error) {
	client := b.Operation.K8sSeedClient
	ecs, err := client.GetSecret(b.Operation.Shoot.SeedNamespace, common.EtcdEncryptionSecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil, false, nil
		} else {
			return false, nil, false, err
		}
	}
	secretData, ok := ecs.Data[common.EtcdEncryptionSecretFileName]
	if !ok {
		return true, nil, false, fmt.Errorf("EncryptionConfiguration in seed cluster (%v) does not contain expected element: %v", common.EtcdEncryptionSecretName, common.EtcdEncryptionSecretFileName)
	}
	ec, err = encryptionconfiguration.CreateFromYAML(secretData)
	if err != nil {
		return true, nil, false, fmt.Errorf("EncryptionConfiguration in seed cluster (%v) is not consistent: %v", common.EtcdEncryptionSecretName, err)
	}
	_, ok = ecs.Annotations[common.EtcdEncryptionForcePlaintextAnnotationName]
	if !ok {
		return true, ec, false, nil
	}
	return true, ec, true, nil
}

func (b *Botanist) calculateEtcdEncryptionSecretNameInGardenCluster() string {
	secretName := fmt.Sprintf("%s.%s", b.Shoot.Info.Name, common.EtcdEncryptionSecretName)
	return secretName
}

// readEncryptionConfigurationFromGarden reads the EncryptionConfiguration from the shoot namespace in the seed
func (b *Botanist) readEncryptionConfigurationFromGarden() (bool, *apiserverconfigv1.EncryptionConfiguration, error) {
	client := b.Operation.K8sGardenClient
	secretNameInGardenCluster := b.calculateEtcdEncryptionSecretNameInGardenCluster()
	ecs, err := client.GetSecret(b.Shoot.Info.Namespace, secretNameInGardenCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil, nil
		} else {
			return false, nil, err
		}
	}
	secretData, ok := ecs.Data[common.EtcdEncryptionSecretFileName]
	if !ok {
		return true, nil, fmt.Errorf("EncryptionConfiguration in garden cluster (%v) does not contain expected element: %v", secretNameInGardenCluster, common.EtcdEncryptionSecretFileName)
	}
	ec, err := encryptionconfiguration.CreateFromYAML(secretData)
	if err != nil {
		return true, nil, fmt.Errorf("EncryptionConfiguration in garden cluster (%v) is not consistent: %v", secretNameInGardenCluster, err)
	}
	return true, ec, nil
}

// writeEncryptionConfiguration writes the secret which contains the EncryptionConfiguration to the
// shoot namespace in the seed cluster as well as to the garden cluster
func (b *Botanist) writeEncryptionConfiguration(ecYamlBytes []byte) error {
	err := b.writeEncryptionConfigurationSecretToSeed(ecYamlBytes)
	if err != nil {
		return err
	}
	err = b.writeEncryptionConfigurationSecretToGarden(ecYamlBytes)
	if err != nil {
		return err
	}
	return nil
}

// writeEncryptionConfigurationSecretToSeed writes the secret which contains the EncryptionConfiguration
// to the shoot namespace in the seed cluster
func (b *Botanist) writeEncryptionConfigurationSecretToSeed(ecYamlBytes []byte) error {
	b.Logger.Info("starting writeEncryptionConfigurationSecretToSeed")

	client := b.Operation.K8sSeedClient
	secretObj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.EtcdEncryptionSecretName,
			Namespace: b.Operation.Shoot.SeedNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			common.EtcdEncryptionSecretFileName: ecYamlBytes,
		},
	}
	if _, err := client.CreateSecretObject(secretObj, true); err != nil {
		return err
	}
	b.Logger.Info("EncryptionConfiguration written to seed successfully")
	return nil
}

// writeEncryptionConfigurationSecretToGarden writes the secret which contains the EncryptionConfiguration
// to the garden cluster
func (b *Botanist) writeEncryptionConfigurationSecretToGarden(ecYamlBytes []byte) error {
	b.Logger.Info("starting writeEncryptionConfigurationSecretToGarden")
	client := b.Operation.K8sGardenClient
	secretNameInGardenCluster := b.calculateEtcdEncryptionSecretNameInGardenCluster()
	secretObj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretNameInGardenCluster,
			Namespace: b.Shoot.Info.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(b.Shoot.Info, gardenv1beta1.SchemeGroupVersion.WithKind("Shoot")),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			common.EtcdEncryptionSecretFileName: ecYamlBytes,
		},
	}
	if _, err := client.CreateSecretObject(secretObj, true); err != nil {
		return err
	}
	b.Logger.Info("EncryptionConfiguration written to garden successfully")
	return nil
}

// isEncryptionConfigurationConsistent checks whether the configuration is consistent in seed and garden
func (b *Botanist) isEncryptionConfigurationConsistent(ecSeed *apiserverconfigv1.EncryptionConfiguration, ecGarden *apiserverconfigv1.EncryptionConfiguration) (bool, error) {
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
func (b *Botanist) needToRewriteShootSecrets() (needToRewriteSecrets bool, err error) {
	client := b.Operation.K8sSeedClient
	ecs, err := client.GetSecret(b.Operation.Shoot.SeedNamespace, common.EtcdEncryptionSecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, fmt.Errorf("no EncryptionConfiguration found in seed cluster (%v)", b.Operation.Shoot.SeedNamespace)
		} else {
			return false, err
		}
	}
	if ecs.Annotations == nil {
		// no annotations found, rewrite secrets
		return true, nil
	}
	lastSecretRewriteChecksum, ok := ecs.Annotations[common.EtcdEncryptionChecksumAnnotationName]
	if !ok {
		// no annotation found, rewrite secrets
		return true, nil
	}
	if b.CheckSums[common.EtcdEncryptionSecretName] == lastSecretRewriteChecksum {
		// no change in checksum, no need to rewrite secrets
		return false, nil
	} else {
		// checksum changed, do rewrite
		return true, nil
	}
}

// rewriteShootSecrets rewrites all secrets of the shoot.
// This will take into account the current EncryptionConfiguration.
func (b *Botanist) rewriteShootSecrets() error {
	client := b.Operation.K8sShootClient
	secretList, err := client.ListSecrets(metav1.NamespaceAll, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, secret := range secretList.Items {
		secretName := secret.GetName()
		secretNamespace := secret.GetNamespace()
		attempt := 0
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			attempt++
			_, updateErr := client.UpdateSecretObject(&secret)
			if updateErr != nil {
				b.Logger.Errorf("attempt %d failed to rewrite secret %v/%v: %v", attempt, secretNamespace, secretName, updateErr)
				return updateErr
			}
			b.Logger.Infof("successfully rewrote secret %v/%v", secretNamespace, secretName)
			return nil
		})
		if err != nil {
			b.Logger.Errorf("failed to rewrite secret %v/%v after %d attempts: %v", secretNamespace, secretName, attempt, err)
		}
	}
	// remember checksum of EncryptionConfiguration used for rewriting shoot secrets
	// as annotation of EncryptionConfiguration
	client = b.Operation.K8sSeedClient
	ecs, err := client.GetSecret(b.Operation.Shoot.SeedNamespace, common.EtcdEncryptionSecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("no EncryptionConfiguration found in seed cluster (%v)", b.Operation.Shoot.SeedNamespace)
		} else {
			return fmt.Errorf("error occurred when reading EncryptionConfiguration in seed cluster (%v): %v", b.Operation.Shoot.SeedNamespace, err)
		}
	}
	if ecs.Annotations == nil {
		ecs.Annotations = make(map[string]string, 1)
	}
	ecs.Annotations[common.EtcdEncryptionChecksumAnnotationName] = b.CheckSums[common.EtcdEncryptionSecretName]
	_, err = client.UpdateSecretObject(ecs)
	if err != nil {
		return err
	}
	return nil
}
