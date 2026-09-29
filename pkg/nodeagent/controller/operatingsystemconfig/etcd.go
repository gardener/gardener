// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	staticpodtranslator "github.com/gardener/gardener/pkg/gardenadm/staticpod"
	"github.com/gardener/gardener/pkg/utils"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (r *Reconciler) generateNodeSpecificETCDCertificates(ctx context.Context, secretsManager secretsmanager.Interface) error {
	machineIP, err := r.machineIP()
	if err != nil {
		return err
	}

	etcdRoleToTLSSecretsMap := make(etcdRoleToTLSSecrets, 2)

	for _, role := range []string{v1beta1constants.ETCDRoleMain, v1beta1constants.ETCDRoleEvents} {
		var (
			etcdName = etcd.Name(role)
			dnsNames = etcd.ClientServiceDNSNames(etcdName, metav1.NamespaceSystem, true)
			tls      = etcdTLSSecrets{}
		)

		tls.server, err = etcd.GenerateServerCertificate(ctx, secretsManager, role, dnsNames, machineIP)
		if err != nil {
			return fmt.Errorf("failed to generate server secret for %s: %w", etcdName, err)
		}

		tls.peer, err = etcd.GeneratePeerCertificate(ctx, secretsManager, role, dnsNames, machineIP)
		if err != nil {
			return fmt.Errorf("failed to generate peer secret for %s: %w", etcdName, err)
		}

		etcdRoleToTLSSecretsMap[role] = tls
	}

	return etcdRoleToTLSSecretsMap.writeToDisk(r.FS)
}

// LookupIP is an alias for net.LookupIP that can be overridden in tests.
var LookupIP = net.LookupIP

// machineIP returns the IP address of the current machine. It prefers IPv6 addresses if indicated in the config, yet
// falling back to any available address.
// Similar to https://github.com/kubernetes/kubernetes/blob/ec9f0d55360f74337f9ef40879434a063821ff5b/pkg/kubelet/nodestatus/setters.go#L162-L178
func (r *Reconciler) machineIP() (net.IP, error) {
	addrs, err := LookupIP(r.HostName)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup IPs for hostname %s: %w", r.HostName, err)
	}

	if ip := utils.IPv4OrIPv6(ptr.Deref(r.Config.PreferIPv6, false), addrs...); ip != nil {
		return ip, nil
	}

	return nil, fmt.Errorf("no IP address found for node")
}

type etcdTLSSecrets struct {
	server, peer *corev1.Secret
}

type etcdRoleToTLSSecrets map[string]etcdTLSSecrets

func (e etcdRoleToTLSSecrets) writeToDisk(fs afero.Afero) error {
	for role, tlsSecrets := range e {
		for volumeName, secret := range map[string]*corev1.Secret{
			etcdconstants.VolumeNameServerTLS: tlsSecrets.server,
			etcdconstants.VolumeNamePeerTLS:   tlsSecrets.peer,
		} {
			dir := staticpodtranslator.HostPath(etcd.Name(role), volumeName)
			if err := fs.MkdirAll(dir, os.ModeDir); err != nil {
				return fmt.Errorf("failed creating directory %s: %w", dir, err)
			}

			for key, value := range secret.Data {
				path := filepath.Join(dir, key)
				if err := fs.WriteFile(path, value, 0640); err != nil {
					return fmt.Errorf("failed to write file to %s: %w", path, err)
				}
			}
		}
	}

	return nil
}
