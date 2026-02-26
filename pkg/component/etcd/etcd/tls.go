// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// GenerateServerCertificate generates the server certificate for the etcd cluster.
func GenerateServerCertificate(
	ctx context.Context,
	secretsManager secretsmanager.Interface,
	role string,
	dnsNames []string,
	ip net.IP,
) (
	*corev1.Secret,
	error,
) {
	return secretsManager.Generate(ctx,
		&secretsutils.CertificateSecretConfig{
			Name:                        secretNamePrefixServer + role + ipSuffix(ip),
			CommonName:                  "etcd-server",
			DNSNames:                    dnsNames,
			IPAddresses:                 clientServiceIPAddresses(ip),
			CertType:                    secretsutils.ServerClientCert,
			SkipPublishingCACertificate: true,
		},
		secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCD, secretsmanager.LoadMissingCAFromCluster(ctx)),
		secretsmanager.Rotate(secretsmanager.InPlace),
	)
}

// GenerateClientCertificate generates the client certificate for the etcd cluster.
func GenerateClientCertificate(ctx context.Context, secretsManager secretsmanager.Interface) (*corev1.Secret, error) {
	return secretsManager.Generate(ctx,
		&secretsutils.CertificateSecretConfig{
			Name:                        SecretNameClient,
			CommonName:                  "etcd-client",
			CertType:                    secretsutils.ClientCert,
			SkipPublishingCACertificate: true,
		},
		secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCD),
		secretsmanager.Rotate(secretsmanager.InPlace),
	)
}

// GenerateServerAndClientCertificates generates both client and server certificates for the etcd cluster.
func GenerateServerAndClientCertificates(
	ctx context.Context,
	secretsManager secretsmanager.Interface,
	role string,
	dnsNames []string,
	ip net.IP,
) (
	etcdCASecret *corev1.Secret,
	serverSecret *corev1.Secret,
	clientSecret *corev1.Secret,
	err error,
) {
	var found bool
	etcdCASecret, found = secretsManager.Get(v1beta1constants.SecretNameCAETCD)
	if !found {
		return nil, nil, nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCD)
	}

	serverSecret, err = GenerateServerCertificate(ctx, secretsManager, role, dnsNames, ip)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate server certificate: %w", err)
	}

	clientSecret, err = GenerateClientCertificate(ctx, secretsManager)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate client certificate: %w", err)
	}

	return etcdCASecret, serverSecret, clientSecret, nil
}

// GeneratePeerCertificate generates the peer certificate for the etcd cluster.
func GeneratePeerCertificate(
	ctx context.Context,
	secretsManager secretsmanager.Interface,
	role string,
	dnsNames []string,
	ip net.IP,
) (
	*corev1.Secret,
	error,
) {
	return secretsManager.Generate(ctx,
		&secretsutils.CertificateSecretConfig{
			Name:                        secretNamePrefixPeerServer + role + ipSuffix(ip),
			CommonName:                  "etcd-server",
			DNSNames:                    dnsNames,
			IPAddresses:                 clientServiceIPAddresses(ip),
			CertType:                    secretsutils.ServerClientCert,
			SkipPublishingCACertificate: true,
		},
		secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCDPeer, secretsmanager.UseCurrentCA, secretsmanager.LoadMissingCAFromCluster(ctx)),
		secretsmanager.Rotate(secretsmanager.InPlace),
	)
}

func ipSuffix(ip net.IP) string {
	if len(ip) == 0 {
		return ""
	}
	return "-" + ip.String()
}

// ClientServiceDNSNames returns the DNS names for the ETCD.
func ClientServiceDNSNames(name, namespace string, runsAsStaticPod bool) []string {
	var domainNames []string
	domainNames = append(domainNames, fmt.Sprintf("%s-local", name))
	domainNames = append(domainNames, kubernetesutils.DNSNamesForService(fmt.Sprintf("%s-client", name), namespace)...)

	// The peer service needs to be considered here since the etcd-backup-restore side-car
	// connects to member pods via pod domain names (e.g. for defragmentation).
	// See https://github.com/gardener/etcd-backup-restore/issues/494
	domainNames = append(domainNames, kubernetesutils.DNSNamesForService(fmt.Sprintf("*.%s-peer", name), namespace)...)

	if runsAsStaticPod {
		domainNames = append(domainNames, "localhost")
	}

	return domainNames
}

func clientServiceIPAddresses(ip net.IP) []net.IP {
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	if len(ip) > 0 {
		ips = append(ips, ip)
	}
	return ips
}

func (e *etcd) peerServiceDNSNames() []string {
	return append(
		kubernetesutils.DNSNamesForService(fmt.Sprintf("%s-peer", e.etcd.Name), e.namespace),
		kubernetesutils.DNSNamesForService(fmt.Sprintf("*.%s-peer", e.etcd.Name), e.namespace)...,
	)
}
