// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package certificates

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionswebhookshoot "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	"github.com/gardener/gardener/pkg/controllerutils"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	"github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	// SeedWebhookConfig is the webhook configuration to reconcile in the Seed cluster.
	SeedWebhookConfig *admissionregistrationv1.MutatingWebhookConfiguration
	// ShootWebhookConfig is the webhook configuration to reconcile in all Shoot clusters.
	ShootWebhookConfig *admissionregistrationv1.MutatingWebhookConfiguration
	// AtomicShootWebhookConfig is an atomic value in which this reconciler will store the updated ShootWebhookConfig.
	// It is supposed to be shared with the ControlPlane actuator. I.e. if the CA bundle changes, this reconciler
	// updates the CA bundle in this value so that the ControlPlane actuator can configure the correct (the new) CA
	// bundle for newly created shoots by loading this value.
	AtomicShootWebhookConfig *atomic.Value
	// CASecretName is the CA config name.
	CASecretName string
	// ServerSecretName is the server certificate config name.
	ServerSecretName string
	// Namespace where the server certificate secret should be located in.
	Namespace string
	// Identity of the secrets manager used for managing the secret.
	Identity string
	// Name of the extension.
	ExtensionName string
	// ShootWebhookManagedResourceName is the name of the ManagedResource containing the raw extensionswebhookshoot webhook config.
	ShootWebhookManagedResourceName string
	// ShootNamespaceSelector is a label selector for extensionswebhookshoot namespaces relevant to the extension.
	ShootNamespaceSelector map[string]string
	// Mode is the webhook client config mode.
	Mode string
	// URL is the URL that is used to register the webhooks in Kubernetes.
	URL string

	serverPort int
	client     client.Client
}

// AddToManager generates webhook CA and server cert if it doesn't exist on the cluster yet. Then it adds reconciler to
// the given manager in order to periodically regenerate the webhook secrets.
func (r *reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	r.serverPort = mgr.GetWebhookServer().Port
	r.client = mgr.GetClient()

	present, err := isWebhookServerSecretPresent(ctx, mgr.GetAPIReader(), r.ServerSecretName, r.Namespace, r.Identity)
	if err != nil {
		return err
	}

	// if webhook CA and server cert have not been generated yet, we need to generate them for the first time now,
	// otherwise the webhook server will not be able to start (which is a non-leader election runnable and is therefore
	// started before this controller)
	if !present {
		// cache is not started yet, we need an uncached client for the initial setup
		uncachedClient, err := client.NewDelegatingClient(client.NewDelegatingClientInput{
			Client:      r.client,
			CacheReader: mgr.GetAPIReader(),
		})
		if err != nil {
			return fmt.Errorf("failed to create new unchached client: %w", err)
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
		RecoverPanic: true,
		// if going into exponential backoff, wait at most the configured sync period
		RateLimiter: workqueue.NewWithMaxWaitRateLimiter(workqueue.DefaultControllerRateLimiter(), r.SyncPeriod),
	})
	if err != nil {
		return err
	}

	return ctrl.Watch(controllerutils.EnqueueOnce, nil)
}

// Reconcile generates new certificates if needed and updates all webhook configurations.
func (r *reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	sm, err := r.newSecretsManager(ctx, log, r.client)
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

	if r.SeedWebhookConfig != nil {
		if err := r.reconcileSeedWebhookConfig(ctx, caBundleSecret); err != nil {
			return reconcile.Result{}, fmt.Errorf("error reconciling seed webhook config: %w", err)
		}
		log.Info("Updated seed webhook config with new CA bundle", "webhookConfig", r.SeedWebhookConfig)
	}

	if r.ShootWebhookConfig != nil {
		// update shoot webhook config object (in memory) with the freshly created CA bundle which is also used by the
		// ControlPlane actuator
		if err := webhook.InjectCABundleIntoWebhookConfig(r.ShootWebhookConfig, caBundleSecret.Data[secretutils.DataKeyCertificateBundle]); err != nil {
			return reconcile.Result{}, err
		}
		r.AtomicShootWebhookConfig.Store(r.ShootWebhookConfig.DeepCopy())

		// reconcile all shoot webhook configs with the freshly created CA bundle
		if err := extensionswebhookshoot.ReconcileWebhooksForAllNamespaces(ctx, r.client, r.ExtensionName, r.ShootWebhookManagedResourceName, r.ShootNamespaceSelector, r.serverPort, r.ShootWebhookConfig); err != nil {
			return reconcile.Result{}, fmt.Errorf("error reconciling all shoot webhook configs: %w", err)
		}
		log.Info("Updated all shoot webhook configs with new CA bundle", "webhookConfig", r.ShootWebhookConfig)
	}

	if err := sm.Cleanup(ctx); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}

func (r *reconciler) reconcileSeedWebhookConfig(ctx context.Context, caBundleSecret *corev1.Secret) error {
	// copy object so that we don't lose its name on API/client errors
	config := r.SeedWebhookConfig.DeepCopyObject().(client.Object)
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(config), config); err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(config.DeepCopyObject().(client.Object), client.MergeFromWithOptimisticLock{})
	if err := webhook.InjectCABundleIntoWebhookConfig(config, caBundleSecret.Data[secretutils.DataKeyCertificateBundle]); err != nil {
		return err
	}
	return r.client.Patch(ctx, config, patch)
}

func isWebhookServerSecretPresent(ctx context.Context, c client.Reader, secretName, namespace, identity string) (bool, error) {
	secretList := &metav1.PartialObjectMetadataList{}
	secretList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))

	if err := c.List(ctx, secretList, client.InNamespace(namespace), client.Limit(1), client.MatchingLabels{
		secretsmanager.LabelKeyName:            secretName,
		secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
		secretsmanager.LabelKeyManagerIdentity: identity,
	}); err != nil {
		return false, err
	}

	return len(secretList.Items) > 0, nil
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
	return sm.Generate(ctx, getWebhookServerCertConfig(r.ServerSecretName, r.Namespace, r.ExtensionName, r.Mode, r.URL),
		secretsmanager.SignedByCA(r.CASecretName, secretsmanager.UseCurrentCA))
}
