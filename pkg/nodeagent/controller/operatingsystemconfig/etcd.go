// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	staticpodtranslator "github.com/gardener/gardener/pkg/gardenadm/staticpod"
)

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
