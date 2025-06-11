// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificates

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/gardener/gardener/pkg/controllerutils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const certificateReloaderName = "webhook-certificate-reloader"

// reloader is a simple reconciler that retrieves the current webhook server certificate managed by a secrets manager
// every syncPeriod and writes it to certDir.
type reloader struct {
	// SyncPeriod is the frequency with which to reload the server cert. Defaults to 5m.
	SyncPeriod time.Duration
	// ServerSecretName is the server certificate config name.
	ServerSecretName string
	// Namespace where the server certificate secret is located in.
	Namespace string
	// Identity of the secrets manager used for managing the secret.
	Identity string

	lock                   sync.Mutex
	reader                 client.Reader
	certDir                string
	newestServerSecretName string
}

// AddToManager does an initial retrieval of an existing webhook server secret and then adds reloader to the given
// manager in order to periodically reload the secret from the cluster.
func (r *reloader) AddToManager(ctx context.Context, mgr manager.Manager, sourceCluster cluster.Cluster) error {
	r.reader = mgr.GetClient()
	if sourceCluster != nil {
		r.reader = sourceCluster.GetClient()
	}

	webhookServer := mgr.GetWebhookServer()
	defaultServer, ok := webhookServer.(*webhook.DefaultServer)
	if !ok {
		return fmt.Errorf("expected *webhook.DefaultServer, got %T", webhookServer)
	}
	r.certDir = defaultServer.Options.CertDir

	// initial retrieval of server cert, needed in order for the webhook server to start successfully
	apiReader := mgr.GetAPIReader()
	if sourceCluster != nil {
		apiReader = sourceCluster.GetAPIReader()
	}

	found, _, serverCert, serverKey, err := r.getServerCert(ctx, apiReader)
	if err != nil {
		return err
	}

	if !found {
		// if we can't find a server cert secret on startup, the leader has not yet generated one
		// exit and retry on next restart
		return fmt.Errorf("couldn't find webhook server secret with name %q managed by secrets manager %q in namespace %q", r.ServerSecretName, r.Identity, r.Namespace)
	}

	if err = writeCertificatesToDisk(r.certDir, serverCert, serverKey); err != nil {
		return err
	}

	opts := controller.Options{
		Reconciler:   r,
		RecoverPanic: ptr.To(true),
		// if going into exponential backoff, wait at most the configured sync period
		RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](), r.SyncPeriod),
	}
	opts.DefaultFromConfig(mgr.GetControllerOptions())

	// add controller that reloads the server cert secret periodically
	ctrl, err := controller.NewUnmanaged(certificateReloaderName, opts)
	if err != nil {
		return err
	}

	if err = ctrl.Watch(controllerutils.EnqueueOnce); err != nil {
		return err
	}

	// we need to run this controller in all replicas even if they aren't leader right now, so that webhook servers
	// in stand-by replicas reload rotated server certificates as well
	return mgr.Add(nonLeaderElectionRunnable{ctrl})
}

// Reconcile reloads the server certificates from the cluster and writes them to the cert directory if they have
// changed. From here, the controller-runtime's certwatcher will pick them up and use them for the webhook server.
func (r *reloader) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx).WithValues(
		"secretConfigName", r.ServerSecretName,
		"secretNamespace", r.Namespace,
		"identity", r.Identity,
		"certDir", r.certDir,
	)

	log.V(1).Info("Reloading server certificate from secret")

	found, secretName, serverCert, serverKey, err := r.getServerCert(ctx, r.reader)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving secret %q from namespace %q", r.ServerSecretName, r.Namespace)
	}

	if !found {
		log.Info("Couldn't find webhook server secret, retrying")
		return reconcile.Result{Requeue: true}, nil
	}

	log = log.WithValues("secretName", secretName)

	r.lock.Lock()
	defer r.lock.Unlock()

	// prevent unnecessary disk writes
	if secretName == r.newestServerSecretName {
		log.V(1).Info("Secret already written to disk, checking again later")
		return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
	}

	log.Info("Found new secret, writing certificate to disk")
	if err = writeCertificatesToDisk(r.certDir, serverCert, serverKey); err != nil {
		return reconcile.Result{}, err
	}

	r.newestServerSecretName = secretName
	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}

func (r *reloader) getServerCert(ctx context.Context, reader client.Reader) (bool, string, []byte, []byte, error) {
	secretList := &corev1.SecretList{}
	if err := reader.List(ctx, secretList, client.InNamespace(r.Namespace), client.MatchingLabels{
		secretsmanager.LabelKeyName:            r.ServerSecretName,
		secretsmanager.LabelKeyManagedBy:       secretsmanager.LabelValueSecretsManager,
		secretsmanager.LabelKeyManagerIdentity: r.Identity,
	}); err != nil {
		return false, "", nil, nil, err
	}

	if len(secretList.Items) != 1 {
		return false, "", nil, nil, nil
	}

	s := secretList.Items[0]
	return true, s.Name, s.Data[secretsutils.DataKeyCertificate], s.Data[secretsutils.DataKeyPrivateKey], nil
}

type nonLeaderElectionRunnable struct {
	manager.Runnable
}

func (n nonLeaderElectionRunnable) NeedLeaderElection() bool {
	return false
}
