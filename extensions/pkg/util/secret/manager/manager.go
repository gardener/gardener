// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// SecretConfigWithOptions combines a secret config with options that should be used for generating it.
type SecretConfigWithOptions struct {
	// Config contains the secret config to generate.
	Config secretsutils.ConfigInterface
	// Options contains options for generating Config.
	Options []secretsmanager.GenerateOption
}

// SecretsManagerForCluster initializes a new SecretsManager for the given Cluster.
// It takes care about rotating CAs among the given secretConfigs in lockstep with all other shoot cluster CAs.
// It basically makes sure your extension fulfills the requirements for shoot CA rotation when managing secrets with this
// SecretsManager. I.e., it
// - initiates rotation of CAs according to cluster.shoot.status.credentials.rotation.certificateAuthorities.lastInitiationTime
// - keeps old CA secrets during CA rotation
// - removes old CA secrets on Cleanup() if cluster.shoot.status.credentials.rotation.certificateAuthorities.phase == Completing
func SecretsManagerForCluster(ctx context.Context, logger logr.Logger, clock clock.Clock, c client.Client, cluster *extensionscontroller.Cluster, identity string, secretConfigs []SecretConfigWithOptions) (secretsmanager.Interface, error) {
	sm, err := secretsmanager.New(ctx, logger, clock, c, cluster.ObjectMeta.Name, identity, secretsmanager.Config{
		CASecretAutoRotation: false,
		SecretNamesToTimes:   lastSecretRotationStartTimesFromCluster(cluster, secretConfigs),
	})
	if err != nil {
		return nil, err
	}
	return secretsManager{
		Interface: sm,
		cluster:   cluster,
	}, nil
}

// secretsManager wraps another SecretsManager in order to automatically fulfill the CA rotation requirements based on
// the Shoot status from the given Cluster object.
type secretsManager struct {
	secretsmanager.Interface
	cluster *extensionscontroller.Cluster
}

// Generate delegates to the contained SecretsManager but automatically injects the `IgnoreOldSecrets` option if the CA
// rotation phase is `Completing`. It always injects the rotation policy `KeepOld` for all CA configs.
func (a secretsManager) Generate(ctx context.Context, config secretsutils.ConfigInterface, opts ...secretsmanager.GenerateOption) (*corev1.Secret, error) {
	if certConfig, ok := config.(*secretsutils.CertificateSecretConfig); ok && certConfig.CertType == secretsutils.CACert {
		// CAs are always rotated in phases (not in-place)
		opts = append(opts, secretsmanager.Rotate(secretsmanager.KeepOld))
		if v1beta1helper.GetShootCARotationPhase(a.cluster.Shoot.Status.Credentials) == gardencorev1beta1.RotationCompleting {
			// we are completing rotation, cleanup the old CA secret
			opts = append(opts, secretsmanager.IgnoreOldSecrets())
		}
	}
	return a.Interface.Generate(ctx, config, opts...)
}

// lastSecretRotationStartTimesFromCluster creates a map that maps names of secret configs to times.
// If cluster.shoot.status.credentials.certificateAuthorities.lastInitiationTime is set, it adds the time for all given
// CA configs. If it's not set or no CA configs are given the map will be empty.
func lastSecretRotationStartTimesFromCluster(cluster *extensionscontroller.Cluster, secretConfigs []SecretConfigWithOptions) map[string]time.Time {
	var (
		secretNamesToTime    = make(map[string]time.Time)
		caLastInitiationTime *time.Time
	)

	if shootStatus := cluster.Shoot.Status; shootStatus.Credentials != nil && shootStatus.Credentials.Rotation != nil &&
		shootStatus.Credentials.Rotation.CertificateAuthorities != nil &&
		shootStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime != nil {
		timeCopy := shootStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time
		caLastInitiationTime = &timeCopy
	}

	for _, caConfig := range filterCAConfigs(secretConfigs) {
		// bind CA rotation lifecycle to the cluster CA (i.e. rotate in lockstep)
		if caLastInitiationTime != nil {
			secretNamesToTime[caConfig.Config.GetName()] = *caLastInitiationTime
		}
	}

	return secretNamesToTime
}

// filterCAConfigs returns a list of all CA configs contained in the given list.
func filterCAConfigs(secretConfigs []SecretConfigWithOptions) []SecretConfigWithOptions {
	var caConfigs []SecretConfigWithOptions

	for _, config := range secretConfigs {
		switch secretConfig := config.Config.(type) {
		case *secretsutils.CertificateSecretConfig:
			if secretConfig.CertType == secretsutils.CACert {
				caConfigs = append(caConfigs, config)
			}
		}
	}

	return caConfigs
}

// GenerateAllSecrets takes care of generating all secret configs with the given SecretsManager (first CA configs, then
// the rest).
func GenerateAllSecrets(ctx context.Context, sm secretsmanager.Interface, secretConfigs []SecretConfigWithOptions) (map[string]*corev1.Secret, error) {
	deployedSecrets := make(map[string]*corev1.Secret, len(secretConfigs))

	// generate all CAs first (needed to sign other certificate configs)
	for _, config := range filterCAConfigs(secretConfigs) {
		secret, err := sm.Generate(ctx, config.Config, config.Options...)
		if err != nil {
			return nil, err
		}
		deployedSecrets[config.Config.GetName()] = secret
	}

	// now, generate the remaining secrets
	for _, config := range secretConfigs {
		if _, ok := deployedSecrets[config.Config.GetName()]; ok {
			// already generated
			continue
		}

		secret, err := sm.Generate(ctx, config.Config, config.Options...)
		if err != nil {
			return nil, err
		}
		deployedSecrets[config.Config.GetName()] = secret
	}

	return deployedSecrets, nil
}
