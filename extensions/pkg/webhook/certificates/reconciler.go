// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificates

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsshootwebhook "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const certificateReconcilerName = "webhook-certificate"

// reconciler is a simple reconciler that manages the webhook CA and server certificate using a secrets manager.
// It runs Generate for both secret configs followed by Cleanup every SyncPeriod and updates the WebhookConfigurations
// accordingly with the new CA bundle.
type reconciler struct {
	// Clock is the clock.
	Clock clock.Clock
	// SyncPeriod is the frequency with which to reload the server cert. Defaults to 5m.
	SyncPeriod time.Duration
	// SourceWebhookConfigs are the webhook configurations to reconcile in the Source cluster.
	SourceWebhookConfigs extensionswebhook.Configs
	// ShootWebhookConfigs are the webhook configurations to reconcile in all Shoot clusters.
	ShootWebhookConfigs *extensionswebhook.Configs
	// AtomicShootWebhookConfigs is an atomic value in which this reconciler will store the updated ShootWebhookConfigs.
	// It is supposed to be shared with the ControlPlane actuator. I.e. if the CA bundle changes, this reconciler
	// updates the CA bundle in this value so that the ControlPlane actuator can configure the correct (the new) CA
	// bundle for newly created shoots by loading this value.
	AtomicShootWebhookConfigs *atomic.Value
	// CASecretName is the CA config name.
	CASecretName string
	// ServerSecretName is the server certificate config name.
	ServerSecretName string
	// Namespace where the server certificate secret should be located in.
	Namespace string
	// Identity of the secrets manager used for managing the secret.
	Identity string
	// Name of the component.
	ComponentName string
	// ShootWebhookManagedResourceName is the name of the ManagedResource containing the raw shoot webhook config.
	ShootWebhookManagedResourceName string
	// ShootNamespaceSelector is a label selector for shoot namespaces relevant to the extension.
	ShootNamespaceSelector map[string]string
	// Mode is the webhook client config mode.
	Mode string
	// URL is the URL that is used to register the webhooks in Kubernetes.
	URL string

	// client is the client used to update webhook configuration objects.
	client client.Client
	// sourceClient is the client used to manage certificate secrets.
	sourceClient client.Client
}

// AddToManager generates webhook CA and server cert if it doesn't exist on the cluster yet.
// Then it adds the reconciler to the given manager in order to periodically regenerate the webhook secrets.
// An 'sourceCluster' can be optionally passed to let the certificate secrets be managed in a different cluster.
func (r *reconciler) AddToManager(ctx context.Context, mgr manager.Manager, sourceCluster cluster.Cluster) error {
	r.client = mgr.GetClient()
	r.sourceClient = mgr.GetClient()
	apiReader := mgr.GetAPIReader()
	scheme := mgr.GetScheme()

	if sourceCluster != nil {
		r.sourceClient = sourceCluster.GetClient()
		apiReader = sourceCluster.GetAPIReader()
		scheme = sourceCluster.GetScheme()
	}

	present, err := isWebhookServerSecretPresent(ctx, apiReader, scheme, r.ServerSecretName, r.Namespace, r.Identity)
	if err != nil {
		return err
	}

	// if webhook CA and server cert have not been generated yet, we need to generate them for the first time now,
	// otherwise the webhook server will not be able to start (which is a non-leader election runnable and is therefore
	// started before this controller)
	if !present {
		restConfig := mgr.GetConfig()
		if sourceCluster != nil {
			restConfig = sourceCluster.GetConfig()
		}

		// cache is not started yet, we need an uncached client for the initial setup
		uncachedClient, err := client.New(restConfig, client.Options{
			Cache: &client.CacheOptions{
				Reader: apiReader,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create new uncached client: %w", err)
		}

		sm, err := r.newSecretsManager(ctx, mgr.GetLogger(), uncachedClient)
		if err != nil {
			return fmt.Errorf("failed to create new SecretsManager: %w", err)
		}

		if _, err = r.generateWebhookCA(ctx, sm); err != nil {
			return err
		}

		if _, err = r.generateWebhookServerCert(ctx, sm); err != nil {
			return err
		}
	}

	// add controller, that regenerates the CA and server cert secrets periodically
	ctrl, err := controller.New(certificateReconcilerName, mgr, controller.Options{
		Reconciler:   r,
		RecoverPanic: ptr.To(true),
		// if going into exponential backoff, wait at most the configured sync period
		RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](), r.SyncPeriod),
	})
	if err != nil {
		return err
	}

	return ctrl.Watch(controllerutils.EnqueueOnce)
}

// Reconcile generates new certificates if needed and updates all webhook configurations.
func (r *reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	sm, err := r.newSecretsManager(ctx, log, r.sourceClient)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create new SecretsManager: %w", err)
	}

	caSecret, err := r.generateWebhookCA(ctx, sm)
	if err != nil {
		return reconcile.Result{}, err
	}
	caBundleSecret, found := sm.Get(r.CASecretName)
	if !found {
		return reconcile.Result{}, fmt.Errorf("secret %q not found", r.CASecretName)
	}

	log = log.WithValues(
		"secretNamespace", r.Namespace,
		"identity", r.Identity,
		"caSecretName", caSecret.Name,
		"caBundleSecretName", caBundleSecret.Name,
	)
	log.Info("Generated webhook CA")

	serverSecret, err := r.generateWebhookServerCert(ctx, sm)
	if err != nil {
		return reconcile.Result{}, err
	}
	log.Info("Generated webhook server cert", "serverSecretName", serverSecret.Name)

	for _, sourceWebhookConfig := range r.SourceWebhookConfigs.GetWebhookConfigs() {
		if err := r.reconcileSourceWebhookConfig(ctx, sourceWebhookConfig, caBundleSecret); err != nil {
			return reconcile.Result{}, fmt.Errorf("error reconciling source webhook config %s: %w", client.ObjectKeyFromObject(sourceWebhookConfig), err)
		}
		log.Info("Updated source webhook config with new CA bundle", "webhookConfig", sourceWebhookConfig)
	}

	if r.ShootWebhookConfigs != nil && r.ShootWebhookConfigs.HasWebhookConfig() {
		for _, shootWebhookConfig := range r.ShootWebhookConfigs.GetWebhookConfigs() {
			// update shoot webhook config object (in memory) with the freshly created CA bundle which is also used by the
			// ControlPlane actuator
			if err := extensionswebhook.InjectCABundleIntoWebhookConfig(shootWebhookConfig, caBundleSecret.Data[secretsutils.DataKeyCertificateBundle]); err != nil {
				return reconcile.Result{}, err
			}
		}

		r.AtomicShootWebhookConfigs.Store(r.ShootWebhookConfigs.DeepCopy())

		// reconcile all shoot webhook configs with the freshly created CA bundle
		if err := extensionsshootwebhook.ReconcileWebhooksForAllNamespaces(ctx, r.client, r.ShootWebhookManagedResourceName, r.ShootNamespaceSelector, *r.ShootWebhookConfigs); err != nil {
			return reconcile.Result{}, fmt.Errorf("error reconciling all shoot webhook configs: %w", err)
		}

		if r.ShootWebhookConfigs.MutatingWebhookConfig != nil {
			log.Info("Updated all shoot mutating webhook configs with new CA bundle", "webhookConfig", r.ShootWebhookConfigs.MutatingWebhookConfig)
		}
		if r.ShootWebhookConfigs.ValidatingWebhookConfig != nil {
			log.Info("Updated all shoot validating webhook configs with new CA bundle", "webhookConfig", r.ShootWebhookConfigs.ValidatingWebhookConfig)
		}
	}

	if err := sm.Cleanup(ctx); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}

func (r *reconciler) reconcileSourceWebhookConfig(ctx context.Context, sourceWebhookConfig client.Object, caBundleSecret *corev1.Secret) error {
	// copy object so that we don't lose its name on API/client errors
	config := sourceWebhookConfig.DeepCopyObject().(client.Object)
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(config), config); err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(config.DeepCopyObject().(client.Object), client.MergeFromWithOptimisticLock{})
	if err := extensionswebhook.InjectCABundleIntoWebhookConfig(config, caBundleSecret.Data[secretsutils.DataKeyCertificateBundle]); err != nil {
		return err
	}
	return r.client.Patch(ctx, config, patch)
}

func isWebhookServerSecretPresent(ctx context.Context, c client.Reader, scheme *runtime.Scheme, secretName, namespace, identity string) (bool, error) {
	return kubernetesutils.ResourcesExist(ctx, c, &corev1.SecretList{}, scheme, client.InNamespace(namespace), client.MatchingLabels{
		secretsmanager.LabelKeyName:            secretName,
		secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
		secretsmanager.LabelKeyManagerIdentity: identity,
	})
}

func (r *reconciler) newSecretsManager(ctx context.Context, log logr.Logger, c client.Client) (secretsmanager.Interface, error) {
	return secretsmanager.New(
		ctx,
		log.WithName("secretsmanager"),
		r.Clock,
		c,
		r.Namespace,
		r.Identity,
		secretsmanager.Config{CASecretAutoRotation: true},
	)
}

func (r *reconciler) generateWebhookCA(ctx context.Context, sm secretsmanager.Interface) (*corev1.Secret, error) {
	return sm.Generate(ctx, getWebhookCAConfig(r.CASecretName),
		secretsmanager.Rotate(secretsmanager.KeepOld), secretsmanager.IgnoreOldSecretsAfter(24*time.Hour))
}

func (r *reconciler) generateWebhookServerCert(ctx context.Context, sm secretsmanager.Interface) (*corev1.Secret, error) {
	// use current CA for signing server cert to prevent mismatches when dropping the old CA from the webhook config
	return sm.Generate(ctx, getWebhookServerCertConfig(r.ServerSecretName, r.Namespace, r.ComponentName, r.Mode, r.URL),
		secretsmanager.SignedByCA(r.CASecretName, secretsmanager.UseCurrentCA))
}
