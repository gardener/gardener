// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

// InitializeSecretsManagement initializes the secrets management and deploys the required secrets to the shoot
// namespace in the seed.
func (b *Botanist) InitializeSecretsManagement(ctx context.Context) error {
	// Generally, the existing secrets in the shoot namespace in the seeds are the source of truth for the secret
	// manager. Hence, if we restore a shoot control plane then let's fetch the existing data from the ShootState and
	// create corresponding secrets in the shoot namespace in the seed before initializing it. Note that this is
	// explicitly only done in case of restoration to prevent split-brain situations as described in
	// https://github.com/gardener/gardener/issues/5377.
	if b.IsRestorePhase() {
		if err := b.restoreSecretsFromShootStateForSecretsManagerAdoption(ctx); err != nil {
			return err
		}
	}

	taskFns := []flow.TaskFn{
		b.generateCertificateAuthorities,
		b.generateGenericTokenKubeconfig,
		b.reconcileWildcardIngressCertificate,
	}

	if v1beta1helper.ShootEnablesSSHAccess(b.Shoot.GetInfo()) {
		taskFns = append(taskFns, b.generateSSHKeypair)
	} else {
		taskFns = append(taskFns, b.deleteSSHKeypair)
	}

	if b.WantsObservabilityComponents() {
		taskFns = append(taskFns, b.generateObservabilityIngressPassword)
	} else {
		taskFns = append(taskFns, b.deleteObservabilityIngressPassword)
	}

	return flow.Sequential(taskFns...)(ctx)
}

func (b *Botanist) lastSecretRotationStartTimes() map[string]time.Time {
	rotation := make(map[string]time.Time)

	if shootStatus := b.Shoot.GetInfo().Status; shootStatus.Credentials != nil && shootStatus.Credentials.Rotation != nil {
		if shootStatus.Credentials.Rotation.CertificateAuthorities != nil && shootStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime != nil {
			for _, config := range caCertConfigurations(b.Shoot.IsWorkerless, b.Shoot.IsAutonomous()) {
				rotation[config.GetName()] = shootStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time
			}
			// The static token secret contains token for the health check of the kube-apiserver.
			// Hence, let's use the last rotation initiation time of the CA rotation also to rotate the static token secret.
			rotation[kubeapiserver.SecretStaticTokenName] = shootStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time
		}

		if shootStatus.Credentials.Rotation.SSHKeypair != nil && shootStatus.Credentials.Rotation.SSHKeypair.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameSSHKeyPair] = shootStatus.Credentials.Rotation.SSHKeypair.LastInitiationTime.Time
		}

		if shootStatus.Credentials.Rotation.Observability != nil && shootStatus.Credentials.Rotation.Observability.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameObservabilityIngressUsers] = shootStatus.Credentials.Rotation.Observability.LastInitiationTime.Time
		}

		if shootStatus.Credentials.Rotation.ServiceAccountKey != nil && shootStatus.Credentials.Rotation.ServiceAccountKey.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameServiceAccountKey] = shootStatus.Credentials.Rotation.ServiceAccountKey.LastInitiationTime.Time
		}

		if shootStatus.Credentials.Rotation.ETCDEncryptionKey != nil && shootStatus.Credentials.Rotation.ETCDEncryptionKey.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameETCDEncryptionKey] = shootStatus.Credentials.Rotation.ETCDEncryptionKey.LastInitiationTime.Time
		}
	}

	return rotation
}

func (b *Botanist) restoreSecretsFromShootStateForSecretsManagerAdoption(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, v := range b.Shoot.GetShootState().Spec.Gardener {
		entry := v

		if entry.Labels[secretsmanager.LabelKeyManagedBy] != secretsmanager.LabelValueSecretsManager ||
			entry.Type != v1beta1constants.DataTypeSecret {
			continue
		}

		fns = append(fns, func(ctx context.Context) error {
			objectMeta := metav1.ObjectMeta{
				Name:      entry.Name,
				Namespace: b.Shoot.ControlPlaneNamespace,
				Labels:    entry.Labels,
			}

			data := make(map[string][]byte)
			if err := json.Unmarshal(entry.Data.Raw, &data); err != nil {
				return err
			}

			secret := secretsmanager.Secret(objectMeta, data)
			return client.IgnoreAlreadyExists(b.SeedClientSet.Client().Create(ctx, secret))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func caCertConfigurations(isWorkerless, isAutonomous bool) []secretsutils.ConfigInterface {
	certificateSecretConfigs := []secretsutils.ConfigInterface{
		// The CommonNames for CA certificates will be overridden with the secret name by the secrets manager when
		// generated to ensure that each CA has a unique common name. For backwards-compatibility, we still keep the
		// CommonNames here (if we removed them then new CAs would be generated with the next shoot reconciliation
		// without the end-user to explicitly trigger it).
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCACluster, CommonName: "kubernetes", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAClient, CommonName: "kubernetes-client", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCD, CommonName: "etcd", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCDPeer, CommonName: "etcd-peer", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAFrontProxy, CommonName: "front-proxy", CertType: secretsutils.CACert},
	}

	if !isWorkerless {
		certificateSecretConfigs = append(certificateSecretConfigs,
			&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAKubelet, CommonName: "kubelet", CertType: secretsutils.CACert},
			&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAMetricsServer, CommonName: "metrics-server", CertType: secretsutils.CACert},
		)

		if !isAutonomous {
			certificateSecretConfigs = append(certificateSecretConfigs,
				&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAVPN, CommonName: "vpn", CertType: secretsutils.CACert},
			)
		}
	}

	return certificateSecretConfigs
}

func (b *Botanist) caCertGenerateOptionsFor(configName string) []secretsmanager.GenerateOption {
	options := []secretsmanager.GenerateOption{
		secretsmanager.Persist(),
		secretsmanager.Rotate(secretsmanager.KeepOld),
	}

	if v1beta1helper.GetShootCARotationPhase(b.Shoot.GetInfo().Status.Credentials) == gardencorev1beta1.RotationCompleting {
		options = append(options, secretsmanager.IgnoreOldSecrets())
	}

	if configName == v1beta1constants.SecretNameCAClient {
		return options
	}

	// For all CAs other than the client CA we ignore the checksum for the CA secret name due to backwards compatibility
	// reasons in case the CA certificate were never rotated yet. With the first rotation we consider the config
	// checksums since we can now assume that all components are able to cater with it (since we only allow triggering
	// CA rotations after we have adapted all components that might rely on the constant CA secret names).
	// The client CA was only introduced late with https://github.com/gardener/gardener/pull/5779, hence nobody was
	// using it and the config checksum could be considered right away.
	if shootStatus := b.Shoot.GetInfo().Status; shootStatus.Credentials == nil ||
		shootStatus.Credentials.Rotation == nil ||
		shootStatus.Credentials.Rotation.CertificateAuthorities == nil {
		options = append(options, secretsmanager.IgnoreConfigChecksumForCASecretName())
	}

	return options
}

func (b *Botanist) generateCertificateAuthorities(ctx context.Context) error {
	var caClientSecret *corev1.Secret

	for _, config := range caCertConfigurations(b.Shoot.IsWorkerless, b.Shoot.IsAutonomous()) {
		caSecret, err := b.SecretsManager.Generate(ctx, config, b.caCertGenerateOptionsFor(config.GetName())...)
		if err != nil {
			return err
		}
		if config.GetName() == v1beta1constants.SecretNameCAClient {
			caClientSecret = caSecret.DeepCopy()
		}
	}

	caBundleSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	caBundle := caBundleSecret.Data[secretsutils.DataKeyCertificateBundle]

	if err := b.syncShootConfigMapToGarden(
		ctx,
		gardenerutils.ShootProjectConfigMapSuffixCACluster,
		map[string]string{
			v1beta1constants.GardenRole:             v1beta1constants.GardenRoleCACluster,
			v1beta1constants.LabelDiscoveryPublic:   v1beta1constants.DiscoveryShootCA,
			v1beta1constants.LabelShootName:         b.Shoot.GetInfo().Name,
			v1beta1constants.LabelShootUID:          string(b.Shoot.GetInfo().UID),
			v1beta1constants.LabelUpdateRestriction: "true",
		},
		nil,
		map[string]string{secretsutils.DataKeyCertificateCA: string(caBundle)},
	); err != nil {
		return err
	}

	if !b.Shoot.IsWorkerless {
		kubeletCABundleSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCAKubelet)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAKubelet)
		}

		kubeletCABundle := kubeletCABundleSecret.Data[secretsutils.DataKeyCertificateBundle]

		if err := b.syncShootConfigMapToGarden(
			ctx,
			gardenerutils.ShootProjectConfigMapSuffixCAKubelet,
			map[string]string{
				v1beta1constants.GardenRole:             v1beta1constants.GardenRoleCAKubelet,
				v1beta1constants.LabelShootName:         b.Shoot.GetInfo().Name,
				v1beta1constants.LabelShootUID:          string(b.Shoot.GetInfo().UID),
				v1beta1constants.LabelUpdateRestriction: "true",
			},
			nil,
			map[string]string{secretsutils.DataKeyCertificateCA: string(kubeletCABundle)},
		); err != nil {
			return err
		}
	}

	// TODO(petersutter): Remove this code and cleanup Secret after v1.135 has been released. The caBundle is now being stored in a <shootname>.ca-cluster ConfigMap.
	if err := b.syncShootCredentialToGarden(
		ctx,
		gardenerutils.ShootProjectSecretSuffixCACluster,
		map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleCACluster},
		nil,
		map[string][]byte{secretsutils.DataKeyCertificateCA: caBundle},
	); err != nil {
		return err
	}

	return b.syncInternalSecretToGarden(
		ctx,
		gardenerutils.ShootProjectSecretSuffixCAClient,
		map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleCAClient},
		nil,
		caClientSecret.Data,
	)
}

func (b *Botanist) generateGenericTokenKubeconfig(ctx context.Context) error {
	genericTokenKubeconfigSecret, err := tokenrequest.GenerateGenericTokenKubeconfig(ctx, b.SecretsManager, b.Shoot.ControlPlaneNamespace, b.Shoot.ComputeInClusterAPIServerAddress(true))
	if err != nil {
		return err
	}

	cluster := &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: b.Shoot.ControlPlaneNamespace}}
	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, b.SeedClientSet.Client(), cluster, func() error {
		metav1.SetMetaDataAnnotation(&cluster.ObjectMeta, v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName, genericTokenKubeconfigSecret.Name)
		return nil
	})
	return err
}

func (b *Botanist) generateSSHKeypair(ctx context.Context) error {
	sshKeypairSecret, err := b.SecretsManager.Generate(ctx, &secretsutils.RSASecretConfig{
		Name:       v1beta1constants.SecretNameSSHKeyPair,
		Bits:       4096,
		UsedForSSH: true,
	}, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.KeepOld))
	if err != nil {
		return err
	}

	if err := b.syncShootCredentialToGarden(
		ctx,
		gardenerutils.ShootProjectSecretSuffixSSHKeypair,
		map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSSHKeyPair},
		nil,
		sshKeypairSecret.Data,
	); err != nil {
		return err
	}

	if sshKeypairSecretOld, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair, secretsmanager.Old); found {
		if err := b.syncShootCredentialToGarden(
			ctx,
			gardenerutils.ShootProjectSecretSuffixOldSSHKeypair,
			map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSSHKeyPair},
			nil,
			sshKeypairSecretOld.Data,
		); err != nil {
			return err
		}
	}

	return nil
}

func (b *Botanist) generateObservabilityIngressPassword(ctx context.Context) error {
	secret, err := b.SecretsManager.Generate(ctx, &secretsutils.BasicAuthSecretConfig{
		Name:           v1beta1constants.SecretNameObservabilityIngressUsers,
		Format:         secretsutils.BasicAuthFormatNormal,
		Username:       "admin",
		PasswordLength: 32,
	}, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	// TODO(oliver-goetz): Remove `url` when Gardener v1.110 is released.
	annotations := map[string]string{
		"url":            "https://" + b.ComputePlutonoHost(),
		"plutono-url":    "https://" + b.ComputePlutonoHost(),
		"prometheus-url": "https://" + b.ComputePrometheusHost(),
	}

	if b.Shoot.WantsAlertmanager {
		annotations["alertmanager-url"] = "https://" + b.ComputeAlertManagerHost()
	}

	return b.syncShootCredentialToGarden(
		ctx,
		gardenerutils.ShootProjectSecretSuffixMonitoring,
		map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring},
		annotations,
		secret.Data,
	)
}

func (b *Botanist) deleteObservabilityIngressPassword(ctx context.Context) error {
	return b.deleteShootCredentialFromGarden(ctx, gardenerutils.ShootProjectSecretSuffixMonitoring)
}

// quotaExceededRegex is used to check if an error occurred due to infrastructure quota limits.
var quotaExceededRegex = regexp.MustCompile(`(?i)((?:^|[^t]|(?:[^s]|^)t|(?:[^e]|^)st|(?:[^u]|^)est|(?:[^q]|^)uest|(?:[^e]|^)quest|(?:[^r]|^)equest)LimitExceeded|Quotas|Quota.*exceeded|exceeded quota|Quota has been met|QUOTA_EXCEEDED)`)

func (b *Botanist) syncShootCredentialToGarden(
	ctx context.Context,
	nameSuffix string,
	labels map[string]string,
	annotations map[string]string,
	data map[string][]byte,
) error {
	gardenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerutils.ComputeShootProjectResourceName(b.Shoot.GetInfo().Name, nameSuffix),
			Namespace: b.Shoot.GetInfo().Namespace,
		},
	}

	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, b.GardenClient, gardenSecret, func() error {
		gardenSecret.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
		}
		gardenSecret.Annotations = annotations
		gardenSecret.Labels = labels
		gardenSecret.Type = corev1.SecretTypeOpaque
		gardenSecret.Data = data
		return nil
	})

	if err != nil && quotaExceededRegex.MatchString(err.Error()) {
		return v1beta1helper.NewErrorWithCodes(err, gardencorev1beta1.ErrorInfraQuotaExceeded)
	}
	return err
}

func (b *Botanist) syncInternalSecretToGarden(
	ctx context.Context,
	nameSuffix string,
	labels map[string]string,
	annotations map[string]string,
	data map[string][]byte,
) error {
	gardenSecret := &gardencorev1beta1.InternalSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerutils.ComputeShootProjectResourceName(b.Shoot.GetInfo().Name, nameSuffix),
			Namespace: b.Shoot.GetInfo().Namespace,
		},
	}

	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, b.GardenClient, gardenSecret, func() error {
		gardenSecret.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
		}
		gardenSecret.Annotations = annotations
		gardenSecret.Labels = labels
		gardenSecret.Type = corev1.SecretTypeOpaque
		gardenSecret.Data = data
		return nil
	})

	return err
}

func (b *Botanist) syncShootConfigMapToGarden(
	ctx context.Context,
	nameSuffix string,
	labels map[string]string,
	annotations map[string]string,
	data map[string]string,
) error {
	gardenConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerutils.ComputeShootProjectResourceName(b.Shoot.GetInfo().Name, nameSuffix),
			Namespace: b.Shoot.GetInfo().Namespace,
		},
	}

	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, b.GardenClient, gardenConfigMap, func() error {
		gardenConfigMap.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(b.Shoot.GetInfo(), gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
		}
		gardenConfigMap.Annotations = annotations
		gardenConfigMap.Labels = labels
		gardenConfigMap.Data = data
		return nil
	})

	if err != nil && quotaExceededRegex.MatchString(err.Error()) {
		return v1beta1helper.NewErrorWithCodes(err, gardencorev1beta1.ErrorInfraQuotaExceeded)
	}
	return err
}

func (b *Botanist) deleteSSHKeypair(ctx context.Context) error {
	return b.deleteShootCredentialFromGarden(ctx, gardenerutils.ShootProjectSecretSuffixSSHKeypair, gardenerutils.ShootProjectSecretSuffixOldSSHKeypair)
}

func (b *Botanist) deleteShootCredentialFromGarden(ctx context.Context, nameSuffixes ...string) error {
	var secretsToDelete []client.Object
	for _, nameSuffix := range nameSuffixes {
		secretsToDelete = append(secretsToDelete, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gardenerutils.ComputeShootProjectResourceName(b.Shoot.GetInfo().Name, nameSuffix),
				Namespace: b.Shoot.GetInfo().Namespace,
			},
		})
	}

	return kubernetesutils.DeleteObjects(ctx, b.GardenClient, secretsToDelete...)
}

func (b *Botanist) reconcileWildcardIngressCertificate(ctx context.Context) error {
	wildcardCert, err := gardenerutils.GetWildcardCertificate(ctx, b.SeedClientSet.Client())
	if err != nil {
		return err
	}
	if wildcardCert == nil {
		return nil
	}

	// Copy certificate to shoot namespace
	certSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wildcardCert.GetName(),
			Namespace: b.Shoot.ControlPlaneNamespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.SeedClientSet.Client(), certSecret, func() error {
		certSecret.Data = wildcardCert.Data
		return nil
	}); err != nil {
		return err
	}

	b.ControlPlaneWildcardCert = certSecret
	return nil
}

// DeployCloudProviderSecret creates or updates the cloud provider secret in the Shoot namespace
// in the Seed cluster.
func (b *Botanist) DeployCloudProviderSecret(ctx context.Context) error {
	switch credentials := b.Shoot.Credentials.(type) {
	case *securityv1alpha1.WorkloadIdentity:
		shootInfo := b.Shoot.GetInfo()
		shootMeta := securityv1alpha1.ContextObject{
			APIVersion: shootInfo.GroupVersionKind().GroupVersion().String(),
			Kind:       shootInfo.Kind,
			Namespace:  ptr.To(shootInfo.Namespace),
			Name:       shootInfo.Name,
			UID:        shootInfo.UID,
		}

		secret, err := workloadidentity.NewSecret(
			v1beta1constants.SecretNameCloudProvider,
			b.Shoot.ControlPlaneNamespace,
			workloadidentity.For(credentials.Name, credentials.Namespace, credentials.Spec.TargetSystem.Type),
			workloadidentity.WithProviderConfig(credentials.Spec.TargetSystem.ProviderConfig),
			workloadidentity.WithContextObject(shootMeta),
			workloadidentity.WithLabels(map[string]string{v1beta1constants.GardenerPurpose: v1beta1constants.SecretNameCloudProvider}),
		)
		if err != nil {
			return err
		}
		return secret.Reconcile(ctx, b.SeedClientSet.Client())
	case *corev1.Secret:
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: b.Shoot.ControlPlaneNamespace,
			},
		}
		_, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.SeedClientSet.Client(), secret, func() error {
			secret.Annotations = map[string]string{}
			secret.Labels = map[string]string{
				v1beta1constants.GardenerPurpose: v1beta1constants.SecretNameCloudProvider,
			}
			secret.Type = corev1.SecretTypeOpaque
			secret.Data = credentials.Data
			return nil
		})
		return err
	default:
		return fmt.Errorf("unexpected type %T, should be either Secret or WorkloadIdentity", credentials)
	}
}
