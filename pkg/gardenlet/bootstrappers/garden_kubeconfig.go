// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// GardenKubeconfig implements manager.Runnable and can be used to fetch a kubeconfig for the garden cluster.
type GardenKubeconfig struct {
	// RuntimeClient is the runtime cluster client (seed or self-hosted shoot).
	RuntimeClient client.Client
	// Log is a logger.
	Log logr.Logger
	// Config is the gardenlet component configuration.
	Config *gardenletconfigv1alpha1.GardenletConfiguration
	// SelfHostedShootMeta is the NamespacedName of the self-hosted shoot the gardenlet is responsible for (if
	// applicable).
	SelfHostedShootMeta *types.NamespacedName
	// Result is a structure that will be filled with information about the requested kubeconfig. Must be initialized
	// by the caller.
	Result *KubeconfigBootstrapResult
}

// KubeconfigBootstrapResult contains information about the result of the kubeconfig bootstrapping process.
type KubeconfigBootstrapResult struct {
	// Kubeconfig is the kubeconfig that can be used to communicate with the garden cluster.
	Kubeconfig []byte
	// CSRName is the name of the created CertificateSigningRequest. This might be empty when no CSR was created (e.g.,
	// because the kubeconfig already exists).
	CSRName string
	// SeedName is the name of the seed the kubeconfig was requested for. This might be empty when no CSR was created
	// (e.g. because the kubeconfig already exists).
	SeedName string
}

// Start starts the garden kubeconfig bootstrap process. It either uses the provided bootstrap kubeconfig with a
// bootstrap token to create a CertificateSigningRequest for retrieving a client certificate, or it returns the already
// existing kubeconfig (stored in the runtime cluster as secret).
func (g *GardenKubeconfig) Start(ctx context.Context) (err error) {
	if g.Config.GardenClientConnection.KubeconfigSecret != nil {
		g.Result.Kubeconfig, g.Result.CSRName, err = g.getOrBootstrapKubeconfig(ctx)
		if err != nil {
			return err
		}
		g.Result.SeedName = gardenletbootstraputil.GetSeedName(g.Config.SeedConfig)
	} else {
		g.Log.Info("No kubeconfig secret given in the configuration under `.gardenClientConnection.kubeconfigSecret`. Skipping the kubeconfig bootstrap process and certificate rotation")
	}

	if g.Result.Kubeconfig == nil {
		g.Log.Info("Falling back to the kubeconfig specified in the configuration under `.gardenClientConnection.kubeconfig`")

		if len(g.Config.GardenClientConnection.Kubeconfig) > 0 {
			return nil
		}

		return errors.New("the configuration file needs to either specify a Garden API Server kubeconfig under `.gardenClientConnection.kubeconfig` or provide bootstrapping information. " +
			"To configure the Gardenlet for bootstrapping, provide the secret containing the bootstrap kubeconfig under `.gardenClientConnection.kubeconfigSecret` and also the secret name where the created kubeconfig should be stored for further use via`.gardenClientConnection.kubeconfigSecret`")
	}

	var gardenAPIReader client.Reader
	if len(g.Config.GardenClientConnection.GardenClusterCACert) == 0 {
		gardenClient, err := kubernetes.NewClientFromBytes(g.Result.Kubeconfig)
		if err != nil {
			return fmt.Errorf("unable to create garden client from kubeconfig: %w", err)
		}
		gardenAPIReader = gardenClient.APIReader()
	}

	g.Result.Kubeconfig, err = gardenletbootstraputil.UpdateGardenKubeconfigCAIfChanged(ctx, g.Log, gardenAPIReader, g.RuntimeClient, g.Result.Kubeconfig, g.Config.GardenClientConnection)
	if err != nil {
		return fmt.Errorf("error updating CA in garden cluster kubeconfig secret: %w", err)
	}

	return nil
}

var (
	// RequestKubeconfigWithBootstrapClient is an alias for bootstrap.RequestKubeconfigWithBootstrapClient.
	// Exposed for testing.
	RequestKubeconfigWithBootstrapClient = bootstrap.RequestKubeconfigWithBootstrapClient
	// NewClientFromBytes is an alias for kubernetes.NewClientFromBytes.
	// Exposed for testing.
	NewClientFromBytes = kubernetes.NewClientFromBytes
)

// getOrBootstrapKubeconfig retrieves an already existing kubeconfig for the Garden cluster from the Seed or bootstraps a new one
func (g *GardenKubeconfig) getOrBootstrapKubeconfig(ctx context.Context) ([]byte, string, error) {
	kubeconfigKey := kubernetesutils.ObjectKeyFromSecretRef(*g.Config.GardenClientConnection.KubeconfigSecret)
	gardenKubeconfig, err := gardenletbootstraputil.GetKubeconfigFromSecret(ctx, g.RuntimeClient, kubeconfigKey)
	if err != nil {
		return nil, "", err
	}

	log := g.Log.WithValues("kubeconfigSecret", kubeconfigKey)
	if len(gardenKubeconfig) > 0 {
		log.Info("Found kubeconfig generated from bootstrap process. Using it")
		return gardenKubeconfig, "", nil
	}

	log.Info("No kubeconfig from a previous bootstrap found. Starting bootstrap process")

	if g.Config.GardenClientConnection.BootstrapKubeconfig == nil {
		log.Info("Unable to perform kubeconfig bootstrapping. The gardenlet configuration `.gardenClientConnection.bootstrapKubeconfig` is not set")
		return nil, "", nil
	}

	bootstrapKubeconfigKey := kubernetesutils.ObjectKeyFromSecretRef(*g.Config.GardenClientConnection.BootstrapKubeconfig)
	log = log.WithValues("bootstrapKubeconfigSecret", bootstrapKubeconfigKey)

	bootstrapKubeconfig, err := gardenletbootstraputil.GetKubeconfigFromSecret(ctx, g.RuntimeClient, bootstrapKubeconfigKey)
	if err != nil {
		return nil, "", err
	}

	if len(bootstrapKubeconfig) == 0 {
		log.Info("Unable to perform kubeconfig bootstrap. Bootstrap secret does not contain a kubeconfig")
		return nil, "", errors.New("bootstrap secret does not contain a kubeconfig, cannot bootstrap")
	}

	bootstrapClientSet, err := NewClientFromBytes(bootstrapKubeconfig)
	if err != nil {
		return nil, "", fmt.Errorf("unable to bootstrap client from bootstrap kubeconfig: %w", err)
	}

	log.Info("Using provided bootstrap kubeconfig to request signed certificate")
	return RequestKubeconfigWithBootstrapClient(
		ctx,
		log,
		g.RuntimeClient,
		bootstrapClientSet,
		kubeconfigKey,
		bootstrapKubeconfigKey,
		g.Config.SeedConfig,
		g.SelfHostedShootMeta,
		g.Config.GardenClientConnection.KubeconfigValidity.Validity,
	)
}
