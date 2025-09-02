// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// GetWarnings returns warnings for the provided shoot.
func GetWarnings(_ context.Context, shoot, oldShoot *core.Shoot, credentialsRotationInterval time.Duration) []string {
	if shoot == nil {
		return nil
	}

	var warnings []string

	if oldShoot != nil {
		warnings = append(warnings, getWarningsForDueCredentialsRotations(shoot, credentialsRotationInterval)...)
		warnings = append(warnings, getWarningsForIncompleteCredentialsRotation(shoot, credentialsRotationInterval)...)
	}

	// TODO(plkokanov): Remove this after support for Kubernetes v1.32 is dropped.
	// We do not check for the Kubernetes version here because the shoot validation code is called before this
	// and forbids setting .spec.kubernetes.kubeControllerManager.podEvictionTimeout for kubernetes >= v1.33.
	if kubeControllerManager := shoot.Spec.Kubernetes.KubeControllerManager; kubeControllerManager != nil && kubeControllerManager.PodEvictionTimeout != nil {
		warnings = append(warnings, "you are setting the spec.kubernetes.kubeControllerManager.podEvictionTimeout field. The field does not have effect since Kubernetes 1.13 and is forbidden to be set starting from Kubernetes 1.33. Instead, use the spec.kubernetes.kubeAPIServer.(defaultNotReadyTolerationSeconds/defaultUnreachableTolerationSeconds) fields.")
	}

	// TODO(AleksandarSavchev): Remove this after support for Kubernetes v1.33 is dropped.
	// We do not check for the Kubernetes version here because the shoot validation code is called before this
	// and forbids setting the etcd encryption key rotation start and complete annotations for kubernetes >= v1.34.
	if shoot.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.OperationRotateETCDEncryptionKeyStart || shoot.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.OperationRotateETCDEncryptionKeyComplete {
		warnings = append(warnings, fmt.Sprintf("you are setting the operation annotation to %s. This annotation has been deprecated and is forbidden to be set starting from Kubernetes 1.34. Instead, use the %s annotation, which performs a full rotation of the ETCD encryption key.", shoot.Annotations[v1beta1constants.GardenerOperation], v1beta1constants.OperationRotateETCDEncryptionKey))
	}

	if supportedVersion, _ := versionutils.CompareVersions(shoot.Spec.Kubernetes.Version, "<", "1.33"); supportedVersion && shoot.Spec.Kubernetes.ClusterAutoscaler != nil && shoot.Spec.Kubernetes.ClusterAutoscaler.MaxEmptyBulkDelete != nil {
		warnings = append(warnings, "you are setting the spec.kubernetes.clusterAutoscaler.maxEmptyBulkDelete field. The field has been deprecated and is forbidden to be set starting from Kubernetes 1.33. Instead, use the spec.kubernetes.clusterAutoscaler.maxScaleDownParallelism field.")
	}

	if helper.IsLegacyAnonymousAuthenticationSet(shoot.Spec.Kubernetes.KubeAPIServer) {
		warnings = append(warnings, "you are setting the spec.kubernetes.kubeAPIServer.enableAnonymousAuthentication field. The field is deprecated. Using Kubernetes v1.32 and above, please use anonymous authentication configuration. See: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#anonymous-authenticator-configuration")
	}

	kubernetesVersion, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	if err == nil && versionutils.ConstraintK8sGreaterEqual133.Check(kubernetesVersion) && ptr.Deref(shoot.Spec.CloudProfileName, "") != "" {
		warnings = append(warnings, "you are setting the spec.cloudProfileName field. The field is deprecated and will be forcefully set empty starting with Kubernetes 1.34. Use the new spec.cloudProfile.name field instead.")
	}

	if shoot.Spec.SecretBindingName != nil {
		warnings = append(warnings, "spec.secretBindingName is deprecated and will be disallowed starting with Kubernetes 1.34. For migration instructions, see: https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md")
	}

	return warnings
}

func getWarningsForDueCredentialsRotations(shoot *core.Shoot, credentialsRotationInterval time.Duration) []string {
	if !isOldEnough(shoot.CreationTimestamp.Time, credentialsRotationInterval) {
		return nil
	}

	if shoot.Status.Credentials == nil || shoot.Status.Credentials.Rotation == nil {
		return []string{"you should consider rotating the shoot credentials, see https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_credentials_rotation.md#gardener-provided-credentials for details"}
	}

	var (
		rotation = shoot.Status.Credentials.Rotation
		warnings []string
	)

	if rotation.CertificateAuthorities == nil || initiationDue(rotation.CertificateAuthorities.LastInitiationTime, credentialsRotationInterval) {
		warnings = append(warnings, "you should consider rotating the certificate authorities, see https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_credentials_rotation.md#certificate-authorities for details")
	}

	if rotation.ETCDEncryptionKey == nil || initiationDue(rotation.ETCDEncryptionKey.LastInitiationTime, credentialsRotationInterval) {
		warnings = append(warnings, "you should consider rotating the ETCD encryption key, see https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_credentials_rotation.md#etcd-encryption-key for details")
	}

	if (shoot.Spec.Purpose == nil || *shoot.Spec.Purpose != core.ShootPurposeTesting) &&
		(rotation.Observability == nil || initiationDue(rotation.Observability.LastInitiationTime, credentialsRotationInterval)) {
		warnings = append(warnings, "you should consider rotating the observability passwords, see https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_credentials_rotation.md#observability-passwords-for-plutono for details")
	}

	if rotation.ServiceAccountKey == nil || initiationDue(rotation.ServiceAccountKey.LastInitiationTime, credentialsRotationInterval) {
		warnings = append(warnings, "you should consider rotating the ServiceAccount token signing key, see https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_credentials_rotation.md#serviceaccount-token-signing-key for details")
	}

	if helper.ShootEnablesSSHAccess(shoot) && (rotation.SSHKeypair == nil || initiationDue(rotation.SSHKeypair.LastInitiationTime, credentialsRotationInterval)) {
		warnings = append(warnings, "you should consider rotating the SSH keypair, see https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_credentials_rotation.md#ssh-key-pair-for-worker-nodes for details")
	}

	return warnings
}

func getWarningsForIncompleteCredentialsRotation(shoot *core.Shoot, credentialsRotationInterval time.Duration) []string {
	if shoot.Status.Credentials == nil || shoot.Status.Credentials.Rotation == nil {
		return nil
	}

	var (
		warnings                      []string
		recommendedCompletionInterval = credentialsRotationInterval / 3
		rotation                      = shoot.Status.Credentials.Rotation
	)

	// Only consider credentials for which completion must be triggered explicitly by the user. Credentials which are
	// rotated in "one phase" are excluded.
	if rotation.CertificateAuthorities != nil && completionDue(rotation.CertificateAuthorities.LastInitiationFinishedTime, rotation.CertificateAuthorities.LastCompletionTriggeredTime, recommendedCompletionInterval) {
		warnings = append(warnings, completionWarning("certificate authorities", recommendedCompletionInterval))
	}
	if rotation.ETCDEncryptionKey != nil && completionDue(rotation.ETCDEncryptionKey.LastInitiationFinishedTime, rotation.ETCDEncryptionKey.LastCompletionTriggeredTime, recommendedCompletionInterval) {
		warnings = append(warnings, completionWarning("ETCD encryption key", recommendedCompletionInterval))
	}
	if rotation.ServiceAccountKey != nil && completionDue(rotation.ServiceAccountKey.LastInitiationFinishedTime, rotation.ServiceAccountKey.LastCompletionTriggeredTime, recommendedCompletionInterval) {
		warnings = append(warnings, completionWarning("ServiceAccount token signing key", recommendedCompletionInterval))
	}

	return warnings
}

func initiationDue(lastInitiationTime *metav1.Time, threshold time.Duration) bool {
	return lastInitiationTime == nil || isOldEnough(lastInitiationTime.Time, threshold)
}

func completionDue(lastInitiationFinishedTime, lastCompletionTriggeredTime *metav1.Time, threshold time.Duration) bool {
	if lastInitiationFinishedTime == nil {
		return false
	}
	if lastCompletionTriggeredTime != nil && lastCompletionTriggeredTime.UTC().After(lastInitiationFinishedTime.UTC()) {
		return false
	}
	return isOldEnough(lastInitiationFinishedTime.Time, threshold)
}

func isOldEnough(t time.Time, threshold time.Duration) bool {
	return t.UTC().Add(threshold).Before(time.Now().UTC())
}

func completionWarning(credentials string, recommendedCompletionInterval time.Duration) string {
	return fmt.Sprintf("the %s rotation initiation was finished more than %s ago and should be completed", credentials, recommendedCompletionInterval)
}
