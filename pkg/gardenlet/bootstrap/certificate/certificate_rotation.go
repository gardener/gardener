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

package certificate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
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
	logger                 logrus.FieldLogger
	clientMap              clientmap.ClientMap
	seedClient             client.Client
	gardenClientConnection *config.GardenClientConnection
	targetClusterName      string
	seedName               string
	seedSelector           *metav1.LabelSelector
}

// NewCertificateRotation creates a certificate manager that can be used to rotate gardenlet's client certificate for the Garden cluster
func NewCertificateManager(clientMap clientmap.ClientMap, seedClient client.Client, config *config.GardenletConfiguration) *Manager {
	seedName := bootstraputil.GetSeedName(config.SeedConfig)
	gardenletTargetClusterName := bootstraputil.GetTargetClusterName(config.SeedClientConnection)

	return &Manager{
		logger:                 logger.NewFieldLogger(logger.Logger, "certificate-rotation", fmt.Sprintf("seed: %s, target cluster: %s", seedName, gardenletTargetClusterName)),
		clientMap:              clientMap,
		seedClient:             seedClient,
		gardenClientConnection: config.GardenClientConnection,
		seedName:               seedName,
		targetClusterName:      gardenletTargetClusterName,
		seedSelector:           config.SeedSelector,
	}
}

// ScheduleCertificateRotation waits until the currently used Garden cluster client certificate approaches expiration.
// Then requests a new certificate and stores the kubeconfig in a secret (`gardenClientConnection.kubeconfigSecret`) on the Seed.
// the argument is a context.Cancel function to cancel the context of the Gardenlet used for graceful termination after a successful certificate rotation.
// When the new gardenlet pod is started, it uses the rotated certificate stored in the secret in the Seed cluster
func (cr *Manager) ScheduleCertificateRotation(ctx context.Context, gardenletCancel context.CancelFunc, recorder record.EventRecorder) {
	go wait.Until(func() {
		certificateSubject, dnsSANs, ipSANs, certificateExpirationTime, err := waitForCertificateRotation(ctx, cr.logger, cr.seedClient, cr.gardenClientConnection, time.Now)
		if err != nil {
			cr.logger.Errorf("waiting for the certificate rotation failed: %v", err)
			return
		}

		err = retry.Until(ctx, certificateWaitTimeout, func(ctx context.Context) (bool, error) {
			ctxWithTimeout, cancel := context.WithTimeout(ctx, certificateWaitTimeout)
			defer cancel()

			err := rotateCertificate(ctxWithTimeout, cr.logger, cr.clientMap, cr.seedClient, cr.gardenClientConnection, certificateSubject, dnsSANs, ipSANs)
			if err != nil {
				cr.logger.Errorf("certificate rotation failed: %v", err)
				return retry.MinorError(err)
			}
			return retry.Ok()
		})
		if err != nil {
			msg := fmt.Sprintf("Failed to rotate the kubeconfig for the Garden API Server. Certificate expires in %s (%s): %v", certificateExpirationTime.UTC().Sub(time.Now().UTC()).Round(time.Second).String(), certificateExpirationTime.Round(time.Second).String(), err)
			cr.logger.Error(msg)
			seeds, err := cr.getTargetedSeeds(ctx)
			if err != nil {
				cr.logger.Warnf("failed to record event on seeds announcing the failed certificate rotation: %v", err)
				return
			}
			for _, seed := range seeds {
				recorder.Event(&seed, corev1.EventTypeWarning, EventGardenletCertificateRotationFailed, msg)
			}
			return
		}

		cr.logger.Info("Terminating Gardenlet after successful certificate rotation.")
		gardenletCancel()
	}, time.Second, ctx.Done())
}

// waitForCertificateRotation determines and waits for the certificate rotation deadline.
// Reschedules the certificate rotation in case the underlying certificate expiration date has changed in the meanwhile.
func waitForCertificateRotation(ctx context.Context, logger logrus.FieldLogger, seedClient client.Client, gardenClientConnection *config.GardenClientConnection, now func() time.Time) (*pkix.Name, []string, []net.IP, *time.Time, error) {
	cert, err := getCurrentCertificate(ctx, logger, seedClient, gardenClientConnection)
	if err != nil {
		return nil, []string{}, []net.IP{}, nil, err
	}
	deadline := nextRotationDeadline(*cert)
	logger.Infof("Certificate expiration is at %v, rotation deadline is at %v", cert.Leaf.NotAfter, deadline)

	if sleepInterval := deadline.Sub(now()); sleepInterval > 0 {
		logger.Infof("Waiting for next certificate rotation in %d days (%s)", int(sleepInterval.Hours()/24), sleepInterval.Round(time.Second).String())
		// block until certificate rotation or until context is cancelled
		select {
		case <-ctx.Done():
			return nil, []string{}, []net.IP{}, nil, ctx.Err()
		case <-time.After(sleepInterval):
		}
	}

	logger.Infof("Starting the certificate rotation")

	// check the validity of the certificate again. It might have changed
	currentCert, err := getCurrentCertificate(ctx, logger, seedClient, gardenClientConnection)
	if err != nil {
		return nil, []string{}, []net.IP{}, nil, err
	}

	if currentCert.Leaf.NotAfter != cert.Leaf.NotAfter {
		return nil, []string{}, []net.IP{}, nil, fmt.Errorf("the certificates expiration date has been changed. Rescheduling certificate rotation")
	}
	return &currentCert.Leaf.Subject, currentCert.Leaf.DNSNames, currentCert.Leaf.IPAddresses, &currentCert.Leaf.NotAfter, nil
}

func getCurrentCertificate(ctx context.Context, logger logrus.FieldLogger, seedClient client.Client, gardenClientConnection *config.GardenClientConnection) (*tls.Certificate, error) {
	secretName := gardenClientConnection.KubeconfigSecret.Name
	secretNamespace := gardenClientConnection.KubeconfigSecret.Namespace

	gardenKubeconfig, err := bootstraputil.GetKubeconfigFromSecret(ctx, seedClient, gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name)
	if err != nil {
		return nil, err
	}

	if len(gardenKubeconfig) == 0 {
		logger.Infof("secret (%s/%s) on the target cluster does not contain a kubeconfig. Falling back to `gardenClientConnection.Kubeconfig`. The secret's `.data` field should contain a key `kubeconfig` that is mapped to a byte representation of the garden kubeconfig", secretNamespace, secretName)
		// check if there is a locally provided kubeconfig via Gardenlet configuration `gardenClientConnection.Kubeconfig`
		if len(gardenClientConnection.Kubeconfig) == 0 {
			return nil, fmt.Errorf("the secret (%s/%s) on the target cluster does not contain a kubeconfig and there is no fallback kubeconfig specified in `gardenClientConnection.Kubeconfig`. The secret's `.data` field should contain a key `kubeconfig` that is mapped to a byte representation of the garden kubeconfig. Possibly there was an external change to the secret specified in `gardenClientConnection.KubeconfigSecret`. If this error continues, stop the gardenlet, and either configure it with a fallback kubeconfig in `gardenClientConnection.Kubeconfig`, or provide `gardenClientConnection.KubeconfigBootstrap` to bootstrap a new certificate", secretNamespace, secretName)
		}
	}

	// get a rest config from either the `gardenClientConnection.KubeconfigSecret` or from the fallback kubeconfig specified in `gardenClientConnection.Kubeconfig`
	restConfig, err := kubernetes.RESTConfigFromClientConnectionConfiguration(&gardenClientConnection.ClientConnectionConfiguration, gardenKubeconfig)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(restConfig.CertData, restConfig.KeyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse X509 certificate from kubeconfig in secret %s/%s on the target cluster: %v", secretNamespace, secretName, err)
	}

	if len(cert.Certificate) < 1 {
		return nil, fmt.Errorf("the X509 certificate from kubeconfig in secret %s/%s on the target cluster is invalid. No cert/key data found", secretNamespace, secretName)
	}

	certs, err := x509.ParseCertificates(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("the X509 certificate from kubeconfig in secret %s/%s on the target cluster cannot be parsed: %v", secretNamespace, secretName, err)
	}

	if len(certs) < 1 {
		return nil, fmt.Errorf("the X509 certificate from kubeconfig in secret %s/%s on the target cluster is invalid", secretNamespace, secretName)
	}

	cert.Leaf = certs[0]
	return &cert, nil
}

// rotateCertificate uses an already existing garden client (already bootstrapped) to request a new client certificate
// after successful retrieval of the client certificate, updates the secret in the seed with the rotated kubeconfig
func rotateCertificate(ctx context.Context, logger logrus.FieldLogger, clientMap clientmap.ClientMap, seedClient client.Client, gardenClientConnection *config.GardenClientConnection, certificateSubject *pkix.Name, dnsSANs []string, ipSANs []net.IP) error {
	// client to communicate with the Garden API server to create the CSR
	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return err
	}
	gardenCertClient := gardenClient.Kubernetes().CertificatesV1beta1()

	// request new client certificate
	certData, privateKeyData, _, err := RequestCertificate(ctx, logger, gardenCertClient, certificateSubject, dnsSANs, ipSANs)
	if err != nil {
		return err
	}

	logger.Infof("Updating secret (%s/%s) in the target cluster with rotated certificate", gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name)

	_, err = bootstraputil.UpdateGardenKubeconfigSecret(ctx, gardenClient.RESTConfig(), certData, privateKeyData, seedClient, gardenClientConnection)
	if err != nil {
		return fmt.Errorf("unable to update secret (%s/%s) on the target cluster during certificate rotation: %v", gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name, err)
	}

	return nil
}

// getTargetedSeeds returns the Seeds that this Gardenlet is reconciling
func getTargetedSeeds(ctx context.Context, gardenClient client.Client, seedSelector *metav1.LabelSelector, seedName string) ([]gardencorev1beta1.Seed, error) {
	if seedSelector != nil {
		seedLabelSelector, err := metav1.LabelSelectorAsSelector(seedSelector)
		if err != nil {
			return nil, err
		}

		seeds := &gardencorev1beta1.SeedList{}
		err = gardenClient.List(ctx, seeds, client.MatchingLabelsSelector{Selector: seedLabelSelector})
		if err != nil {
			return nil, err
		}
		return seeds.Items, nil
	}

	seed := &gardencorev1beta1.Seed{}
	if err := gardenClient.Get(ctx, client.ObjectKey{Name: seedName}, seed); err != nil {
		return nil, err
	}
	return []gardencorev1beta1.Seed{*seed}, nil
}

func (cr *Manager) getTargetedSeeds(ctx context.Context) ([]gardencorev1beta1.Seed, error) {
	gardenClient, err := cr.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}
	return getTargetedSeeds(ctx, gardenClient.Client(), cr.seedSelector, cr.seedName)
}
