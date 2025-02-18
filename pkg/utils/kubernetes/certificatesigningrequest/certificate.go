// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificatesigningrequest

import (
	"context"
	"crypto"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	certutil "k8s.io/client-go/util/cert"
	csrutil "k8s.io/client-go/util/certificate/csr"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils/retry"
)

// RequestCertificate will create a certificate signing request and send it to API server, then it will watch the object's
// status, once approved, it will return the kube-controller-manager's issued certificate (pem-encoded). If there is any
// errors, or the watch timeouts, it will return an error.
func RequestCertificate(
	ctx context.Context,
	log logr.Logger,
	client kubernetesclientset.Interface,
	certificateSubject *pkix.Name,
	dnsSANs []string,
	ipSANs []net.IP,
	validityDuration *metav1.Duration,
	csrPrefix string,
) (
	[]byte,
	[]byte,
	string,
	error,
) {
	if certificateSubject == nil || len(certificateSubject.CommonName) == 0 {
		return nil, nil, "", fmt.Errorf("unable to request certificate. The Common Name (CN) of the of the certificate Subject has to be set")
	}

	privateKeyData, err := keyutil.MakeEllipticPrivateKeyPEM()
	if err != nil {
		return nil, nil, "", fmt.Errorf("error generating client certificate private key: %w", err)
	}

	certData, csrName, err := requestCertificate(ctx, log, client, privateKeyData, certificateSubject, dnsSANs, ipSANs, validityDuration, csrPrefix)
	if err != nil {
		return nil, nil, "", err
	}
	return certData, privateKeyData, csrName, nil
}

// DigestedName is an alias for certificatesigningrequest.DigestedName.
// Exposed for testing.
var DigestedName = ComputeDigestedName

// ComputeDigestedName is a digest that should include all the relevant pieces of the CSR we care about.
// We can't directly hash the serialized CSR because of random padding that we
// regenerate every loop, and we include usages which are not contained in the
// CSR. This needs to be kept up to date as we add new fields to the node
// certificates and with `ensureCompatible` (https://github.com/kubernetes/client-go/blob/37045084c2aa82927b0e5ffc752861430fd7e4ab/util/certificate/csr/csr.go#L307).
func ComputeDigestedName(publicKey any, subject *pkix.Name, usages []certificatesv1.KeyUsage, csrPrefix string) (string, error) {
	hash := sha512.New512_256()

	// Here we make sure two different inputs can't write the same stream
	// to the hash. This delimiter is not in the base64.URLEncoding
	// alphabet so there is no way to have spill over collisions. Without
	// it 'CN:foo,ORG:bar' hashes to the same value as 'CN:foob,ORG:ar'
	const delimiter = '|'
	encode := base64.RawURLEncoding.EncodeToString

	write := func(data []byte) {
		_, _ = hash.Write([]byte(encode(data)))
		_, _ = hash.Write([]byte{delimiter})
	}

	publicKeyData, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	write(publicKeyData)

	write([]byte(subject.CommonName))
	for _, v := range subject.Organization {
		write([]byte(v))
	}

	for _, v := range usages {
		write([]byte(v))
	}

	return csrPrefix + encode(hash.Sum(nil)), nil
}

// requestCertificate will create a certificate signing request and send it to API server, then it will watch the object's
// status, once approved, it will return the kube-controller-manager's issued certificate (pem-encoded). If there is any
// errors, or the watch timeouts, it will return an error.
func requestCertificate(
	ctx context.Context,
	log logr.Logger,
	client kubernetesclientset.Interface,
	privateKeyData []byte,
	certificateSubject *pkix.Name,
	dnsSANs []string,
	ipSANs []net.IP,
	validityDuration *metav1.Duration,
	csrPrefix string,
) (
	certData []byte,
	csrName string,
	err error,
) {
	privateKey, err := keyutil.ParsePrivateKeyPEM(privateKeyData)
	if err != nil {
		return nil, "", fmt.Errorf("invalid private key for certificate request: %w", err)
	}
	csrData, err := certutil.MakeCSR(privateKey, certificateSubject, dnsSANs, ipSANs)
	if err != nil {
		return nil, "", fmt.Errorf("unable to generate certificate request: %w", err)
	}

	usages := []certificatesv1.KeyUsage{
		certificatesv1.UsageDigitalSignature,
		certificatesv1.UsageKeyEncipherment,
		certificatesv1.UsageClientAuth,
	}

	// The Signer interface contains the Public() method to get the public key.
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, "", fmt.Errorf("private key does not implement crypto.Signer")
	}

	name, err := DigestedName(signer.Public(), certificateSubject, usages, csrPrefix)
	if err != nil {
		return nil, "", err
	}

	var requestedDuration *time.Duration
	if validityDuration != nil {
		requestedDuration = ptr.To(validityDuration.Duration)
	}

	log = log.WithValues("certificateSigningRequestName", name)
	log.Info("Creating certificate signing request")

	reqName, reqUID, err := csrutil.RequestCertificate(client, csrData, name, certificatesv1.KubeAPIServerClientSignerName, requestedDuration, usages, privateKey)
	if err != nil {
		return nil, "", err
	}

	childCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	log.Info("Waiting for certificate signing request to be approved and contain the client certificate")

	certData, err = waitForCertificate(childCtx, client, reqName, reqUID)
	if err != nil {
		return nil, "", err
	}

	log.Info("Certificate signing request got approved. Retrieved client certificate")

	return certData, reqName, nil
}

// waitForCertificate is heavily inspired from k8s.io/client-go/util/certificate/csr.WaitForCertificate. We don't call
// this function directly because it performs LIST/WATCH requests while waiting for the certificate. However, gardenlet
// is only allowed to GET CSR resources related to its seed.
func waitForCertificate(ctx context.Context, client kubernetesclientset.Interface, reqName string, reqUID types.UID) ([]byte, error) {
	var certData []byte

	if err := retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		csr, err := client.CertificatesV1().CertificateSigningRequests().Get(ctx, reqName, metav1.GetOptions{})
		if err != nil {
			return retry.SevereError(fmt.Errorf("failed reading CertificateSigningRequest %s: %w", reqName, err))
		}

		if csr.UID != reqUID {
			return retry.SevereError(fmt.Errorf("csr %q changed UIDs", csr.Name))
		}

		approved := false
		for _, condition := range csr.Status.Conditions {
			if condition.Type == certificatesv1.CertificateDenied {
				return retry.SevereError(fmt.Errorf("CertificateSigningRequest %s is denied, reason: %v, message: %v", csr.Name, condition.Reason, condition.Message))
			}
			if condition.Type == certificatesv1.CertificateFailed {
				return retry.SevereError(fmt.Errorf("CertificateSigningRequest %s failed, reason: %v, message: %v", csr.Name, condition.Reason, condition.Message))
			}
			if condition.Type == certificatesv1.CertificateApproved {
				approved = true
				break
			}
		}

		if !approved {
			return retry.MinorError(fmt.Errorf("CertificateSigningRequest %s is not yet approved, waiting", csr.Name))
		}

		if len(csr.Status.Certificate) == 0 {
			return retry.MinorError(fmt.Errorf("CertificateSigningRequest %s is approved, waiting to be issued", csr.Name))
		}

		certData = csr.Status.Certificate
		return retry.Ok()
	}); err != nil {
		return nil, err
	}

	return certData, nil
}
