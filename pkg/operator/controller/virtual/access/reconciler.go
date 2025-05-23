// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// Reconciler reconciles garden access secrets.
type Reconciler struct {
	Client  client.Client
	FS      afero.Fs
	Channel chan event.TypedGenericEvent[*rest.Config]

	tokenFilePath string
}

// Reconcile processes the given access secret in the request.
// It extracts the included Kubeconfig, and prepares a dedicated REST config
// where the inline bearer token is replaced by a bearer token file.
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

	kubeConfigRaw, ok := secret.Data[kubernetes.KubeConfig]
	if !ok {
		log.Info("Secret does not contain kubeconfig")
		return reconcile.Result{}, nil
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigRaw)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error creating REST config: %w", err)
	}

	if restConfig.BearerToken == "" {
		log.Info("BearerToken has not been populated yet in gardener-operator's virtual cluster kubeconfig")
		return reconcile.Result{}, nil
	}

	if err := r.writeTokenToFile(log, restConfig.BearerToken); err != nil {
		return reconcile.Result{}, fmt.Errorf("error writing bearer token to file: %w", err)
	}

	restConfig.BearerToken = ""
	restConfig.BearerTokenFile = r.tokenFilePath

	log.Info("Notifying virtual cluster creation reconciler about new REST config")
	r.Channel <- event.TypedGenericEvent[*rest.Config]{Object: restConfig}
	return reconcile.Result{}, nil
}

// CreateTemporaryFile creates a temporary file. Exposed for testing.
var CreateTemporaryFile = afero.TempFile

func (r *Reconciler) writeTokenToFile(log logr.Logger, token string) error {
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

	log.Info("Writing virtual garden access token to file", "path", r.tokenFilePath)
	return afero.WriteFile(r.FS, r.tokenFilePath, []byte(token), 0o600)
}
