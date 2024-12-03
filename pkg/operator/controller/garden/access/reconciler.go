// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/operator/controller/gardenlet"
)

// Reconciler reconciles garden access secrets.
type Reconciler struct {
	Client  client.Client
	Manager manager.Manager
	Config  *config.OperatorConfiguration

	FS            afero.Fs
	tokenFilePath string

	GardenletControllerAdded bool
}

// Reconcile processes the given access secret in the request.
// It extracts the included Kubeconfig, and prepares a dedicated REST config
// where the inline Bearer Token is replaced by a BearerTokenFile with the said token persisted.
// Afterward, the config is used to add the 'controller/gardenlet' controller to the manager.
// Any subsequent reconciliation run causes the content of the BearerTokenFile to be updated with the token found
// in the access secret.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, request.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	kubeConfigBytes, ok := secret.Data[kubernetesclient.KubeConfig]
	if !ok {
		log.Info("Secret does not contain kubeconfig")
		return reconcile.Result{}, nil
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigBytes)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error creating REST config: %w", err)
	}

	if restConfig.BearerToken == "" {
		log.Info("BearerToken is not yet populated in kubeconfig")
		return reconcile.Result{}, nil
	}

	if err := r.writeTokenToFile(restConfig.BearerToken); err != nil {
		return reconcile.Result{}, fmt.Errorf("error writing bearer token to file: %w", err)
	}

	if err := r.addGardenletControllerOnce(ctx, log, restConfig); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// CreateTemporaryFile creates a temporary file. Exposed for testing.
var CreateTemporaryFile = afero.TempFile

func (r *Reconciler) writeTokenToFile(token string) error {
	if r.tokenFilePath == "" {
		tokenFile, err := CreateTemporaryFile(r.FS, "", "garden-access")
		if err != nil {
			return fmt.Errorf("error creating gardener-access-kubeconfig file: %w", err)
		}
		r.tokenFilePath = tokenFile.Name()
		if err := tokenFile.Close(); err != nil {
			return fmt.Errorf("error closing gardener-access-kubeconfig file: %w", err)
		}
	}

	return afero.WriteFile(r.FS, r.tokenFilePath, []byte(token), 0o600)
}

func (r *Reconciler) addGardenletControllerOnce(ctx context.Context, log logr.Logger, restConfig *rest.Config) error {
	if r.GardenletControllerAdded {
		return nil
	}

	// prepare REST config
	conf := &kubernetesclient.Config{}
	if err := kubernetesclient.WithRESTConfig(restConfig)(conf); err != nil {
		return fmt.Errorf("error setting rest config: %w", err)
	}
	if err := kubernetesclient.WithClientConnectionOptions(r.Config.VirtualClientConnection)(conf); err != nil {
		return fmt.Errorf("error setting client connection configuration: %w", err)
	}
	restConfig.BearerToken = ""
	restConfig.BearerTokenFile = r.tokenFilePath

	virtualCluster, err := cluster.New(restConfig, func(opts *cluster.Options) {
		opts.Scheme = operatorclient.VirtualScheme
		opts.Logger = log
	})
	if err != nil {
		return fmt.Errorf("could not instantiate virtual cluster: %w", err)
	}
	if err := r.Manager.Add(virtualCluster); err != nil {
		return fmt.Errorf("failed adding virtual cluster to manager: %w", err)
	}

	log.Info("Adding Gardenlet controller to manager now that Garden has been reconciled successfully")
	if err := (&gardenlet.Reconciler{
		Config: r.Config.Controllers.GardenletDeployer,
	}).AddToManager(ctx, r.Manager, virtualCluster); err != nil {
		return fmt.Errorf("failed adding Gardenlet controller: %w", err)
	}
	r.GardenletControllerAdded = true

	return nil
}
