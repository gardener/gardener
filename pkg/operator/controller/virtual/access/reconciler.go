// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package access

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller/tokenrequestor"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// Reconciler reconciles garden access secrets.
type Reconciler struct {
	Client          client.Client
	FS              afero.Fs
	Channel         chan event.TypedGenericEvent[*rest.Config]
	GardenNamespace string
	ConcurrentSyncs int
	APIAudiences    []string
	Clock           clock.Clock

	tokenFilePath string
}

// NewVirtualClient builds a client to the virtual cluster from the given REST config. Exposed for testing.
var NewVirtualClient = func(restConfig *rest.Config) (client.Client, error) {
	return client.New(restConfig, client.Options{Scheme: operatorclient.VirtualScheme})
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

	if restConfig.BearerToken == "" || r.tokenNeedsRenewal(log, secret) {
		log.Info("Bearer token not yet populated or needs renewal, running token-requestor with a short-lived certificated-based kubeconfig to populate it")

		// Build a short-lived, certificate-based REST config that helps us to request a new token from the virtual cluster's token-requestor controller.
		bootstrapConfig := rest.CopyConfig(restConfig)
		if err := r.setBootstrapClientCertificate(ctx, bootstrapConfig); err != nil {
			return reconcile.Result{}, fmt.Errorf("error creating bootstrap client certificate: %w", err)
		}

		virtualClient, err := NewVirtualClient(bootstrapConfig)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("error creating virtual cluster client: %w", err)
		}

		tokenRequestor := &tokenrequestor.Reconciler{
			SourceClient: r.Client,
			TargetClient: virtualClient,
			Clock:        r.Clock,
			JitterFunc:   wait.Jitter,
			APIAudiences: r.APIAudiences,
			CAData:       bootstrapConfig.CAData,
		}
		if _, err := tokenRequestor.Reconcile(ctx, request); err != nil {
			return reconcile.Result{}, fmt.Errorf("error running token-requestor to populate virtual garden token: %w", err)
		}

		log.Info("Token-requestor has successfully populated the virtual garden token, new token will be picked up in the next reconciliation run")
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

// tokenNeedsRenewal reports whether the token in the given access secret has reached the renew timestamp recorded by
// the token-requestor in the 'serviceaccount.resources.gardener.cloud/token-renew-timestamp' annotation. A missing or
// unparsable timestamp is treated as "no renewal needed" (the empty-token check handles the bootstrap case).
func (r *Reconciler) tokenNeedsRenewal(log logr.Logger, secret *corev1.Secret) bool {
	renewTimestamp := secret.Annotations[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]
	if renewTimestamp == "" {
		return false
	}

	renewTime, err := time.Parse(time.RFC3339, renewTimestamp)
	if err != nil {
		log.Error(err, "Could not parse token renew timestamp, skipping renewal check", "renewTimestamp", renewTimestamp)
		return false
	}

	return !r.Clock.Now().UTC().Before(renewTime.UTC())
}

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

// setBootstrapClientCertificate creates a short-lived client certificate signed by the `ca-client` CA
// and wires it into the given REST config. This bootstraps the virtual cluster client before the token-requestor
// controller has populated the gardener-internal secret with a ServiceAccount token.
func (r *Reconciler) setBootstrapClientCertificate(ctx context.Context, restConfig *rest.Config) error {
	caClient, err := r.getCAClientCertificate(ctx)
	if err != nil {
		return err
	}

	clientCert, err := (&secretsutils.CertificateSecretConfig{
		Name:         "virtual-garden-access-bootstrap",
		CommonName:   "gardener.cloud:system:gardener-internal",
		Organization: []string{user.SystemPrivilegedGroup},
		CertType:     secretsutils.ClientCert,
		Validity:     new(10 * time.Minute),
		SigningCA:    caClient,
	}).GenerateCertificate()
	if err != nil {
		return fmt.Errorf("error generating bootstrap client certificate: %w", err)
	}

	restConfig.CertData = clientCert.CertificatePEM
	restConfig.KeyData = clientCert.PrivateKeyPEM
	restConfig.BearerToken = ""
	restConfig.BearerTokenFile = ""

	return nil
}

// getCAClientCertificate loads the current ca-client CA certificate (including its private key) from the runtime
// cluster. The CA is managed by the secrets-manager, so the newest secret (by creation timestamp) carrying the
// 'name=ca-client' and 'managed-by=secrets-manager' labels is the current one.
func (r *Reconciler) getCAClientCertificate(ctx context.Context) (*secretsutils.Certificate, error) {
	secretList := &corev1.SecretList{}
	if err := r.Client.List(ctx, secretList,
		client.InNamespace(r.GardenNamespace),
		client.MatchingLabels{
			secretsmanager.LabelKeyName:      v1beta1constants.SecretNameCAClient,
			secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
		},
	); err != nil {
		return nil, fmt.Errorf("error listing %s CA secrets: %w", v1beta1constants.SecretNameCAClient, err)
	}

	if len(secretList.Items) == 0 {
		return nil, fmt.Errorf("no %s CA secret found in namespace %q", v1beta1constants.SecretNameCAClient, r.GardenNamespace)
	}

	kubernetesutils.ByCreationTimestamp().Sort(secretList)
	caSecret := secretList.Items[len(secretList.Items)-1]

	return secretsutils.LoadCertificate(v1beta1constants.SecretNameCAClient, caSecret.Data[secretsutils.DataKeyPrivateKeyCA], caSecret.Data[secretsutils.DataKeyCertificateCA])
}
