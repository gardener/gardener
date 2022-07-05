// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package app

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getOrBootstrapKubeconfig retrieves an already existing kubeconfig for the Garden cluster from the Seed or bootstraps a new one
func getOrBootstrapKubeconfig(
	ctx context.Context,
	log logr.Logger,
	seedClient client.Client,
	config *config.GardenletConfiguration,
) (
	[]byte,
	string,
	string,
	error,
) {
	kubeconfigKey := kutil.ObjectKeyFromSecretRef(*config.GardenClientConnection.KubeconfigSecret)
	gardenKubeconfig, err := bootstraputil.GetKubeconfigFromSecret(ctx, seedClient, kubeconfigKey)
	if err != nil {
		return nil, "", "", err
	}

	log = log.WithValues("kubeconfigSecret", kubeconfigKey)
	if len(gardenKubeconfig) > 0 {
		log.Info("Found kubeconfig generated from bootstrap process. Using it")
		return gardenKubeconfig, "", "", nil
	}

	log.Info("No kubeconfig from a previous bootstrap found. Starting bootstrap process")

	if config.GardenClientConnection.BootstrapKubeconfig == nil {
		log.Info("Unable to perform kubeconfig bootstrapping. The gardenlet configuration `.gardenClientConnection.bootstrapKubeconfig` is not set")
		return nil, "", "", nil
	}

	bootstrapKubeconfigKey := kutil.ObjectKeyFromSecretRef(*config.GardenClientConnection.BootstrapKubeconfig)
	log.WithValues("bootstrapKubeconfigSecret", bootstrapKubeconfigKey)

	bootstrapKubeconfig, err := bootstraputil.GetKubeconfigFromSecret(ctx, seedClient, bootstrapKubeconfigKey)
	if err != nil {
		return nil, "", "", err
	}

	if len(bootstrapKubeconfig) == 0 {
		log.Info("Unable to perform kubeconfig bootstrap. Bootstrap secret does not contain a kubeconfig")
		return nil, "", "", nil
	}

	bootstrapClientSet, err := kubernetes.NewClientFromBytes(bootstrapKubeconfig)
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to bootstrap client from bootstrap kubeconfig: %w", err)
	}

	seedName := bootstraputil.GetSeedName(config.SeedConfig)

	log = log.WithValues("seedName", seedName)
	log.Info("Using provided bootstrap kubeconfig to request signed certificate")

	return bootstrap.RequestBootstrapKubeconfig(ctx, log, seedClient, bootstrapClientSet, kubeconfigKey, bootstrapKubeconfigKey, seedName)
}
