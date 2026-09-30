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
	"slices"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	staticpodtranslator "github.com/gardener/gardener/pkg/gardenadm/staticpod"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (r *Reconciler) generateNodeSpecificETCDCertificates(ctx context.Context, secretsManager secretsmanager.Interface, osc *extensionsv1alpha1.OperatingSystemConfig) error {
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

			fetchCAFromOSCAndGenerateCertificate = func(
				baseDir string,
				generateFn func(context.Context, secretsmanager.Interface, string, []string, net.IP, ...secretsmanager.SignedByCAOption) (*corev1.Secret, error),
			) (
				*corev1.Secret,
				error,
			) {
				currentCACert, currentCAKey, oldCACert, oldCAKey, err := r.rawCADataFromOperatingSystemConfig(ctx, osc, baseDir)
				if err != nil {
					return nil, err
				}
				return generateFn(ctx, secretsManager, role, dnsNames, machineIP, secretsmanager.LoadMissingCAFromRaw(currentCACert, currentCAKey, oldCACert, oldCAKey))
			}
		)

		tls.server, err = fetchCAFromOSCAndGenerateCertificate(v1beta1constants.OperatingSystemConfigFilePathCAETCD, etcd.GenerateServerCertificate)
		if err != nil {
			return fmt.Errorf("failed to generate server secret for %s: %w", etcdName, err)
		}

		tls.peer, err = fetchCAFromOSCAndGenerateCertificate(v1beta1constants.OperatingSystemConfigFilePathCAETCDPeer, etcd.GeneratePeerCertificate)
		if err != nil {
			return fmt.Errorf("failed to generate peer secret for %s: %w", etcdName, err)
		}

		etcdRoleToTLSSecretsMap[role] = tls
	}

	return etcdRoleToTLSSecretsMap.writeToDisk(r.FS)
}

func (r *Reconciler) rawCADataFromOperatingSystemConfig(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig, baseDir string) (
	currentCert, currentKey []byte,
	oldCert, oldKey []byte,
	err error,
) {
	var (
		dirCurrent = baseDir + v1beta1constants.OperatingSystemConfigFolderCurrent
		dirOld     = baseDir + v1beta1constants.OperatingSystemConfigFolderOld
		found      bool
	)

	currentCert, found, err = r.fileContentForPath(ctx, osc, filepath.Join(dirCurrent, secretsutils.DataKeyCertificateCA))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed looking up ETCD-related CA at %s: %w", dirCurrent, err)
	}
	if !found {
		return nil, nil, nil, nil, fmt.Errorf("current ETCD-related CA not found at %s", dirCurrent)
	}
	currentKey, found, err = r.fileContentForPath(ctx, osc, filepath.Join(dirCurrent, secretsutils.DataKeyPrivateKeyCA))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed looking up ETCD CA key at %s: %w", dirCurrent, err)
	}
	if !found {
		return nil, nil, nil, nil, fmt.Errorf("current ETCD-related CA not found at %s", dirCurrent)
	}

	oldCert, _, err = r.fileContentForPath(ctx, osc, filepath.Join(dirOld, secretsutils.DataKeyCertificateCA))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed looking up ETCD-related CA at %s: %w", dirOld, err)
	}
	oldKey, _, err = r.fileContentForPath(ctx, osc, filepath.Join(dirOld, secretsutils.DataKeyPrivateKeyCA))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed looking up ETCD CA key at %s: %w", dirOld, err)
	}

	return
}

func (r *Reconciler) fileContentForPath(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig, path string) ([]byte, bool, error) {
	idx := slices.IndexFunc(osc.Spec.Files, func(file extensionsv1alpha1.File) bool {
		return file.Path == path
	})
	if idx == -1 {
		return nil, false, nil
	}

	data, ok, err := r.getFileContentData(ctx, osc.Spec.Files[idx])
	if err != nil {
		return nil, true, fmt.Errorf("unable to get data for file with path %s: %w", path, err)
	}
	if !ok {
		return nil, true, fmt.Errorf("unsupported file content type for file with path %s", path)
	}

	return data, true, nil
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
