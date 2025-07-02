// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

// ComputeOperationType determines which operation should be executed when acting on the given shoot.
func ComputeOperationType(shoot *gardencorev1beta1.Shoot) gardencorev1beta1.LastOperationType {
	if v1beta1helper.ShouldPrepareShootForMigration(shoot) {
		return gardencorev1beta1.LastOperationTypeMigrate
	}

	lastOperation := shoot.Status.LastOperation
	if lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		(lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded || lastOperation.State == gardencorev1beta1.LastOperationStateAborted) {
		return gardencorev1beta1.LastOperationTypeRestore
	}

	return v1beta1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation)
}

// GetEtcdDeployTimeout returns the timeout for the etcd deployment task of the reconcile flow.
func GetEtcdDeployTimeout(shoot *shoot.Shoot, defaultDuration time.Duration) time.Duration {
	timeout := defaultDuration
	if v1beta1helper.IsHAControlPlaneConfigured(shoot.GetInfo()) {
		timeout = etcd.DefaultTimeout
	}
	return timeout
}

// StartRotationETCDEncryptionKey start the etcd encryption key rotation by mutating rotation status.
func StartRotationETCDEncryptionKey(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootETCDEncryptionKeyRotation(shoot, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationPreparing
		rotation.LastInitiationTime = now
		rotation.LastInitiationFinishedTime = now
		rotation.LastCompletionTriggeredTime = nil
	})
}

// StartRotationSSHKeypair start the ssh keypair rotation by mutating rotation status.
func StartRotationSSHKeypair(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootSSHKeypairRotation(shoot, func(rotation *gardencorev1beta1.ShootSSHKeypairRotation) {
		rotation.LastInitiationTime = now
	})
}

// StartRotationObservability start the observability passwords rotation by mutating rotation status.
func StartRotationObservability(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateObservabilityRotation(shoot, func(rotation *gardencorev1beta1.ObservabilityRotation) {
		rotation.LastInitiationTime = now
	})
}
