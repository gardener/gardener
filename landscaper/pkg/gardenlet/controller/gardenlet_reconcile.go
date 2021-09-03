// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/landscaper/pkg/gardenlet/chart"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// Reconcile deploys the Gardenlet into the Seed cluster
func (g *Landscaper) Reconcile(ctx context.Context) error {
	seedConfig := g.gardenletConfiguration.SeedConfig

	// deploy the Seed secret containing the Seed cluster kubeconfig to the Garden cluster
	// only deployed if secret reference is explicitly configured in the Seed resource in the import configuration
	if g.gardenletConfiguration.SeedConfig.Spec.SecretRef != nil {
		seedKubeconfig := g.imports.SeedCluster.Spec.Configuration.RawMessage
		if g.gardenletConfiguration.SeedClientConnection != nil && len(g.gardenletConfiguration.SeedClientConnection.Kubeconfig) > 0 {
			seedKubeconfig = []byte(g.gardenletConfiguration.SeedClientConnection.Kubeconfig)
		}

		if err := g.deploySeedSecret(ctx, seedKubeconfig, g.gardenletConfiguration.SeedConfig.Spec.SecretRef); err != nil {
			return fmt.Errorf("failed to deploy secret for the Seed resource containing the Seed cluster's kubeconfig: %w", err)
		}
	}

	// if configured, deploy the seed-backup secret to the Garden cluster
	if g.imports.SeedBackupCredentials != nil && seedConfig.Spec.Backup != nil {
		credentials := make(map[string][]byte)
		marshalJSON, err := g.imports.SeedBackupCredentials.MarshalJSON()
		if err != nil {
			return err
		}

		if err := json.Unmarshal(marshalJSON, &credentials); err != nil {
			return err
		}

		if err := g.deployBackupSecret(ctx, seedConfig.Spec.Backup.Provider, credentials, seedConfig.Spec.Backup.SecretRef); err != nil {
			return fmt.Errorf("failed to deploy the Seed backup secret to the Garden cluster: %w", err)
		}
	}

	var (
		isAlreadyBootstrapped = true
		err                   error
	)

	// This check is to determine whether the Gardenlet needs bootstrap credentials.
	// It is not enough to just check for the existence of the Seed resource to verify that the current Gardenlet
	// has already valid Garden cluster credentials.
	// The Gardenlet might have failed to rotate its credentials and fails to reconcile the Seed.
	// Possibly, this deploys bootstrap credentials even though they are not required in order to increase reliability
	// this should not be a problem, as the landscaper always uses bootstrap secrets with a limited lifetime.
	// CSR is approved by GCM, which is not available during integration tests - skip.
	if !g.isIntegrationTest {
		isAlreadyBootstrapped, err = g.isSeedBootstrapped(ctx, seedConfig.ObjectMeta)
		if err != nil {
			return fmt.Errorf("failed to check if seed %q is already bootstrapped: %w", seedConfig.ObjectMeta.Name, err)
		}
	}

	var bootstrapKubeconfig []byte
	if !isAlreadyBootstrapped {
		bootstrapKubeconfig, err = g.getKubeconfigWithBootstrapToken(ctx, seedConfig.ObjectMeta)
		if err != nil {
			return fmt.Errorf("failed to compute the bootstrap kubeconfig: %w", err)
		}
	}

	gardenNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.GardenNamespace}}
	if err := g.seedClient.Client().Create(ctx, gardenNamespace); kutil.IgnoreAlreadyExists(err) != nil {
		return err
	}

	values, err := g.computeGardenletChartValues(bootstrapKubeconfig)
	if err != nil {
		return fmt.Errorf("failed to compute gardenlet chart values: %w", err)
	}

	applier := chart.NewGardenletChartApplier(g.seedClient.ChartApplier(), values, g.chartPath)
	if err := applier.Deploy(ctx); err != nil {
		return fmt.Errorf("failed deploying Gardenlet chart to the Seed cluster: %w", err)
	}

	g.log.Infof("Successfully deployed Gardenlet resources for Seed %q", g.gardenletConfiguration.SeedConfig.Name)

	return g.waitForRolloutToBeComplete(ctx)
}

func (g *Landscaper) waitForRolloutToBeComplete(ctx context.Context) error {
	g.log.Info("Waiting for the Gardenlet to be rolled out successfully...")

	// sleep for couple seconds to give the gardenlet process time to startup and either fail or proceed
	// otherwise the Seed might be observed as READY, only because the deployed Gardenlet
	// has not reconciled it yet
	time.Sleep(g.rolloutSleepDuration)

	var (
		deploymentRolloutSuccessful bool
		seedIsRegistered            bool
	)

	return retry.UntilTimeout(ctx, 10*time.Second, 15*time.Minute, func(ctx context.Context) (done bool, err error) {
		gardenletDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardenlet",
				Namespace: v1beta1constants.GardenNamespace,
			},
		}

		err = g.seedClient.Client().Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: v1beta1constants.DeploymentNameGardenlet}, gardenletDeployment)
		if err != nil {
			return retry.SevereError(err)
		}

		// in integration tests, we do not assume that the Gardenlet can be rolled out successfully,
		// nor that the Seed can be registered
		// this is to provide an easy means of testing the landscaper component without requiring
		// a fully functional Gardener control plane
		if g.isIntegrationTest {
			return retry.Ok()
		}

		if err := health.CheckDeployment(gardenletDeployment); err != nil {
			msg := fmt.Sprintf("gardenlet deployment is not rolled out successfuly yet...: %v", err)
			g.log.Info(msg)
			return retry.MinorError(fmt.Errorf(msg))
		}

		if !deploymentRolloutSuccessful {
			g.log.Info("Gardenlet deployment rollout successful!")
			deploymentRolloutSuccessful = true
		}

		seed := &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: g.gardenletConfiguration.SeedConfig.Name,
			},
		}

		err = g.gardenClient.Client().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed)
		if err != nil {
			if apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("seed %q is not yet registered...: %v", seed.Name, err)
				g.log.Info(msg)
				return retry.MinorError(fmt.Errorf(msg))
			}
			return retry.SevereError(err)
		}

		if !seedIsRegistered {
			g.log.Infof("Seed %q is registered!", seed.Name)
			seedIsRegistered = true
		}

		// Check Seed without caring for the observing Gardener version (unknown to the landscaper)
		err = health.CheckSeed(seed, seed.Status.Gardener)
		if err != nil {
			msg := fmt.Sprintf("Seed %q is not yet ready...: %v", seed.Name, err)
			g.log.Info(msg)
			return retry.MinorError(fmt.Errorf(msg))
		}
		g.log.Info("Seed is bootstrapped and ready!")
		return retry.Ok()
	})
}

func (g *Landscaper) getKubeconfigWithBootstrapToken(ctx context.Context, seedObjectMeta metav1.ObjectMeta) ([]byte, error) {
	var (
		tokenID     = bootstraputil.TokenID(seedObjectMeta)
		description = bootstraputil.Description(bootstraputil.KindSeed, "", seedObjectMeta.Name)
		validity    = 24 * time.Hour
	)
	return bootstraputil.ComputeGardenletKubeconfigWithBootstrapToken(ctx, g.gardenClient.Client(), g.gardenClient.RESTConfig(), tokenID, description, validity)
}

// isSeedBootstrapped checks is the Seed reconciled by this Gardenlet is exists and is healthy
func (g *Landscaper) isSeedBootstrapped(ctx context.Context, seedMeta metav1.ObjectMeta) (bool, error) {
	seed := &gardencorev1beta1.Seed{ObjectMeta: seedMeta}
	if err := g.gardenClient.Client().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Check Seed without caring for the observing Gardener version (unknown to the landscaper)
	err := health.CheckSeed(seed, seed.Status.Gardener)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func (g *Landscaper) deploySeedSecret(ctx context.Context, runtimeClusterKubeconfig []byte, secretRef *corev1.SecretReference) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretRef.Name,
			Namespace: secretRef.Namespace,
		},
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, g.gardenClient.Client(), secret, func() error {
		secret.Data = map[string][]byte{
			kubernetes.KubeConfig: runtimeClusterKubeconfig,
		}
		secret.Type = corev1.SecretTypeOpaque
		return nil
	})
	return err
}

func (g *Landscaper) deployBackupSecret(ctx context.Context, providerName string, credentials map[string][]byte, backupSecretRef corev1.SecretReference) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupSecretRef.Name,
			Namespace: backupSecretRef.Namespace,
		},
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, g.gardenClient.Client(), secret, func() error {
		secret.Data = credentials
		secret.Type = corev1.SecretTypeOpaque
		secret.Labels = map[string]string{
			"provider": providerName,
		}
		return nil
	})
	return err
}
