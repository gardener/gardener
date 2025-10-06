// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificate

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	// certificateWaitTimeout controls the amount of time we wait for the certificate
	// approval in one iteration.
	certificateWaitTimeout = 15 * time.Minute

	// EventGardenletCertificateRotationFailed is an event reason to describe a failed Gardenlet certificate rotation.
	EventGardenletCertificateRotationFailed = "GardenletCertificateRotationFailed"
)

// Manager can be used to schedule the certificate rotation for the Gardenlet's Garden cluster client certificate
type Manager struct {
	log                    logr.Logger
	gardenClientSet        kubernetes.Interface
	seedClient             client.Client
	gardenClientConnection *gardenletconfigv1alpha1.GardenClientConnection
	newTargetedObject      func() client.Object
}

// NewCertificateManager creates a certificate manager that can be used to rotate gardenlet's client certificate for the Garden cluster
func NewCertificateManager(
	log logr.Logger,
	gardenCluster cluster.Cluster,
	seedClient client.Client,
	config *gardenletconfigv1alpha1.GardenletConfiguration,
	autonomousShootMeta *types.NamespacedName,
) (
	*Manager,
	error,
) {
	gardenClientSet, err := kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(gardenCluster.GetConfig()),
		kubernetes.WithRuntimeAPIReader(gardenCluster.GetAPIReader()),
		kubernetes.WithRuntimeClient(gardenCluster.GetClient()),
		kubernetes.WithRuntimeCache(gardenCluster.GetCache()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating garden clientset: %w", err)
	}

	logger := log.WithName("certificate-manager")
	if autonomousShootMeta != nil {
		logger = logger.WithValues("shootNamespace", autonomousShootMeta.Namespace, "shootName", autonomousShootMeta.Name)
	} else {
		logger = logger.WithValues("seedName", gardenletbootstraputil.GetSeedName(config.SeedConfig))
	}

	return &Manager{
		log:                    logger,
		gardenClientSet:        gardenClientSet,
		seedClient:             seedClient,
		gardenClientConnection: config.GardenClientConnection,
		newTargetedObject: func() client.Object {
			if autonomousShootMeta != nil {
				return &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: autonomousShootMeta.Namespace, Name: autonomousShootMeta.Name}}
			}
			return &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: gardenletbootstraputil.GetSeedName(config.SeedConfig)}}
		},
	}, nil
}

// ScheduleCertificateRotation waits until the currently used Garden cluster client certificate approaches expiration.
// Then requests a new certificate and stores the kubeconfig in a secret (`gardenClientConnection.kubeconfigSecret`) on the Seed.
// the argument is a context.Cancel function to cancel the context of the Gardenlet used for graceful termination after a successful certificate rotation.
// When the new gardenlet pod is started, it uses the rotated certificate stored in the secret in the Seed cluster
func (cr *Manager) ScheduleCertificateRotation(ctx context.Context, gardenletCancel context.CancelFunc, recorder record.EventRecorder) error {
	wait.Until(func() {
		certificateSubject, dnsSANs, ipSANs, certificateExpirationTime, err := waitForCertificateRotation(ctx, cr.log, cr.seedClient, cr.gardenClientConnection, time.Now)
		if err != nil {
			cr.log.Error(err, "Waiting for the certificate rotation failed")
			return
		}

		if err := retry.Until(ctx, 10*time.Second, func(ctx context.Context) (bool, error) {
			ctxWithTimeout, cancel := context.WithTimeout(ctx, certificateWaitTimeout)
			defer cancel()

			if err := rotateCertificate(ctxWithTimeout, cr.log, cr.gardenClientSet, cr.seedClient, cr.gardenClientConnection, certificateSubject, dnsSANs, ipSANs); err != nil {
				cr.log.Error(err, "Certificate rotation failed")
				return retry.MinorError(err)
			}
			return retry.Ok()
		}); err != nil {
			cr.log.Error(err, "Failed to rotate the kubeconfig for the Garden API Server", "certificateExpirationTime", certificateExpirationTime)
			obj, err := cr.getTargetedObject(ctx)
			if err != nil {
				cr.log.Error(err, "Failed to record event on my Seed or Shoot object announcing the failed certificate rotation")
				return
			}
			recorder.Event(obj, corev1.EventTypeWarning, EventGardenletCertificateRotationFailed, fmt.Sprintf("Failed to rotate the kubeconfig for the Garden API Server. Certificate expires in %s (%s): %v", certificateExpirationTime.UTC().Sub(time.Now().UTC()).Round(time.Second).String(), certificateExpirationTime.Round(time.Second).String(), err))
			return
		}

		cr.log.Info("Terminating gardenlet after successful certificate rotation")
		gardenletCancel()
	}, time.Second, ctx.Done())
	return nil
}

// getTargetedObject returns the Seed or the autonomous Shoot that this Gardenlet is reconciling.
func (cr *Manager) getTargetedObject(ctx context.Context) (client.Object, error) {
	obj := cr.newTargetedObject()
	if err := cr.gardenClientSet.Client().Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// waitForCertificateRotation determines and waits for the certificate rotation deadline.
// Reschedules the certificate rotation in case the underlying certificate expiration date has changed in the meanwhile.
func waitForCertificateRotation(
	ctx context.Context,
	log logr.Logger,
	seedClient client.Client,
	gardenClientConnection *gardenletconfigv1alpha1.GardenClientConnection,
	now func() time.Time,
) (
	*pkix.Name,
	[]string,
	[]net.IP,
	*time.Time,
	error,
) {
	kubeconfigSecret, cert, err := readCertificateFromKubeconfigSecret(ctx, log, seedClient, gardenClientConnection)
	if err != nil {
		return nil, []string{}, []net.IP{}, nil, err
	}

	deadline := nextRotationDeadline(*cert, gardenClientConnection.KubeconfigValidity)
	log.Info("Determined certificate expiration and rotation deadline", "notAfter", cert.Leaf.NotAfter, "rotationDeadline", deadline)

	if sleepInterval := deadline.Sub(now()); sleepInterval > 0 {
		log.Info("Waiting for next certificate rotation", "interval", sleepInterval)
	}

	var stopWaiting bool
	for !stopWaiting {
		if kubeconfigSecret.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.KubeconfigSecretOperationRenew {
			log.Info("Certificate expiration has not passed but immediate renewal was requested", "notAfter", cert.Leaf.NotAfter)
			return &cert.Leaf.Subject, cert.Leaf.DNSNames, cert.Leaf.IPAddresses, &cert.Leaf.NotAfter, nil
		}

		select {
		case <-ctx.Done(): // context is cancelled
			return nil, []string{}, []net.IP{}, nil, ctx.Err()

		case <-time.After(deadline.Sub(now())): // certificate rotation is due
			stopWaiting = true

		case <-time.After(time.Second * 10): // check every 10 seconds for immediate renewal request
			var tmpCert *tls.Certificate
			kubeconfigSecret, tmpCert, err = readCertificateFromKubeconfigSecret(ctx, log, seedClient, gardenClientConnection)
			if err != nil {
				return nil, []string{}, []net.IP{}, nil, err
			}
			if tmpCert.Leaf.NotAfter != cert.Leaf.NotAfter {
				stopWaiting = true
			}
		}
	}

	log.Info("Starting the certificate rotation")

	// check the validity of the certificate again. It might have changed
	_, currentCert, err := readCertificateFromKubeconfigSecret(ctx, log, seedClient, gardenClientConnection)
	if err != nil {
		return nil, []string{}, []net.IP{}, nil, err
	}

	if currentCert.Leaf.NotAfter != cert.Leaf.NotAfter {
		return nil, []string{}, []net.IP{}, nil, fmt.Errorf("the certificates expiration date has been changed. Rescheduling certificate rotation")
	}

	return &currentCert.Leaf.Subject, currentCert.Leaf.DNSNames, currentCert.Leaf.IPAddresses, &currentCert.Leaf.NotAfter, nil
}

func readCertificateFromKubeconfigSecret(ctx context.Context, log logr.Logger, seedClient client.Client, gardenClientConnection *gardenletconfigv1alpha1.GardenClientConnection) (*corev1.Secret, *tls.Certificate, error) {
	kubeconfigSecret := &corev1.Secret{}
	if err := seedClient.Get(ctx, client.ObjectKey{Namespace: gardenClientConnection.KubeconfigSecret.Namespace, Name: gardenClientConnection.KubeconfigSecret.Name}, kubeconfigSecret); client.IgnoreNotFound(err) != nil {
		return nil, nil, err
	}

	cert, err := GetCurrentCertificate(log, kubeconfigSecret.Data[kubernetes.KubeConfig], gardenClientConnection)
	if err != nil {
		return nil, nil, err
	}

	return kubeconfigSecret, cert, nil
}

// GetCurrentCertificate returns the client certificate which is currently used to communicate with the garden cluster.
func GetCurrentCertificate(log logr.Logger, gardenKubeconfig []byte, gardenClientConnection *gardenletconfigv1alpha1.GardenClientConnection) (*tls.Certificate, error) {
	kubeconfigKey := kubernetesutils.ObjectKeyFromSecretRef(*gardenClientConnection.KubeconfigSecret)
	log = log.WithValues("kubeconfigSecret", kubeconfigKey)

	if len(gardenKubeconfig) == 0 {
		log.Info("Kubeconfig secret on the target cluster does not contain a kubeconfig. Falling back to `gardenClientConnection.Kubeconfig`. The secret's `.data` field should contain a key `kubeconfig` that is mapped to a byte representation of the garden kubeconfig")
		// check if there is a locally provided kubeconfig via Gardenlet configuration `gardenClientConnection.Kubeconfig`
		if len(gardenClientConnection.Kubeconfig) == 0 {
			return nil, fmt.Errorf("the kubeconfig secret %q on the target cluster does not contain a kubeconfig and there is no fallback kubeconfig specified in `gardenClientConnection.Kubeconfig`. The secret's `.data` field should contain a key `kubeconfig` that is mapped to a byte representation of the garden kubeconfig. Possibly there was an external change to the secret specified in `gardenClientConnection.KubeconfigSecret`. If this error continues, stop the gardenlet, and either configure it with a fallback kubeconfig in `gardenClientConnection.Kubeconfig`, or provide `gardenClientConnection.KubeconfigBootstrap` to bootstrap a new certificate", kubeconfigKey.String())
		}
	}

	// get a rest config from either the `gardenClientConnection.KubeconfigSecret` or from the fallback kubeconfig specified in `gardenClientConnection.Kubeconfig`
	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&gardenClientConnection.ClientConnectionConfiguration, gardenKubeconfig)
	if err != nil {
		return nil, err
	}

	return kubernetesutils.ClientCertificateFromRESTConfig(restConfig)
}

// rotateCertificate uses an already existing garden client (already bootstrapped) to request a new client certificate
// after successful retrieval of the client certificate, updates the secret in the seed with the rotated kubeconfig
func rotateCertificate(
	ctx context.Context,
	log logr.Logger,
	gardenClientSet kubernetes.Interface,
	seedClient client.Client,
	gardenClientConnection *gardenletconfigv1alpha1.GardenClientConnection,
	certificateSubject *pkix.Name,
	dnsSANs []string,
	ipSANs []net.IP,
) error {
	// request new client certificate
	certData, privateKeyData, _, err := certificatesigningrequest.RequestCertificate(ctx, log, gardenClientSet.Kubernetes(), certificateSubject, dnsSANs, ipSANs, gardenClientConnection.KubeconfigValidity.Validity, bootstrap.SeedCSRPrefix)
	if err != nil {
		return err
	}

	kubeconfigKey := kubernetesutils.ObjectKeyFromSecretRef(*gardenClientConnection.KubeconfigSecret)
	log = log.WithValues("kubeconfigSecret", kubeconfigKey)
	log.Info("Updating kubeconfig secret in target cluster with rotated certificate")

	_, err = gardenletbootstraputil.UpdateGardenKubeconfigSecret(ctx, gardenClientSet.RESTConfig(), certData, privateKeyData, seedClient, kubeconfigKey)
	if err != nil {
		return fmt.Errorf("unable to update kubeconfig secret %q on the target cluster during certificate rotation: %w", kubeconfigKey.String(), err)
	}

	return nil
}
