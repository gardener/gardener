// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// SnapshotEtcd performs a full snapshot on ETCD main.
func SnapshotEtcd(ctx context.Context, secretsManager secretsmanager.Interface, etcdMain etcd.Interface) error {
	etcdCASecret, found := secretsManager.Get(v1beta1constants.SecretNameCAETCD)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCD)
	}

	caCerts := x509.NewCertPool()
	caCerts.AppendCertsFromPEM(etcdCASecret.Data[secretsutils.DataKeyCertificateBundle])

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caCerts,
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return etcdMain.Snapshot(ctx, httpClient)
}
