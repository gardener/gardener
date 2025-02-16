// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GetWarnings returns warnings for the provided shoot.
func GetWarnings(_ context.Context, shoot, oldShoot *core.Shoot, credentialsRotationInterval time.Duration) []string {
	if shoot == nil {
		return nil
	}

	var warnings []string

	if ptr.Deref(shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig, true) {
		warnings = append(warnings, "you should consider disabling the static token kubeconfig, see https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_access.md for details")
	}

	if oldShoot != nil {
		warnings = append(warnings, getWarningsForDueCredentialsRotations(shoot, credentialsRotationInterval)...)
		warnings = append(warnings, getWarningsForIncompleteCredentialsRotation(shoot, credentialsRotationInterval)...)
	}

	if kubeControllerManager := shoot.Spec.Kubernetes.KubeControllerManager; kubeControllerManager != nil && kubeControllerManager.PodEvictionTimeout != nil {
		warnings = append(warnings, "you are setting the spec.kubernetes.kubeControllerManager.podEvictionTimeout field. The field does not have effect since Kubernetes 1.13. Instead, use the spec.kubernetes.kubeAPIServer.(defaultNotReadyTolerationSeconds/defaultUnreachableTolerationSeconds) fields.")
	}

	if metav1.HasAnnotation(shoot.ObjectMeta, v1beta1constants.AnnotationManagedSeedAPIServer) && shoot.Namespace == v1beta1constants.GardenNamespace {
		warnings = append(warnings, "annotation 'shoot.gardener.cloud/managed-seed-api-server' is deprecated, instead consider enabling high availability for the ManagedSeed's Shoot control plane")
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

	if ptr.Deref(shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig, true) &&
		(rotation.Kubeconfig == nil || initiationDue(rotation.Kubeconfig.LastInitiationTime, credentialsRotationInterval)) {
		warnings = append(warnings, "you should consider rotating the static token kubeconfig, see https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_credentials_rotation.md#kubeconfig for details")
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
	if lastCompletionTriggeredTime != nil && lastCompletionTriggeredTime.Time.UTC().After(lastInitiationFinishedTime.Time.UTC()) {
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
