// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
)

const nodeAgentCSRPrefix = "node-agent-csr-"

// RequestAndStoreKubeconfig requests a certificate via CSR and stores the resulting kubeconfig on the disk.
func RequestAndStoreKubeconfig(ctx context.Context, log logr.Logger, fs afero.Afero, config *rest.Config, machineName string) error {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	certificateSubject := &pkix.Name{
		Organization: []string{v1beta1constants.NodeAgentsGroup},
		CommonName:   v1beta1constants.NodeAgentUserNamePrefix + machineName,
	}

	certData, privateKeyData, _, err := certificatesigningrequest.RequestCertificate(ctx, log, clientSet, certificateSubject, []string{}, []net.IP{}, &metav1.Duration{Duration: time.Hour * 720}, nodeAgentCSRPrefix)
	if err != nil {
		return fmt.Errorf("unable to request the client certificate for the gardener-node-agent kubeconfig: %w", err)
	}

	// Get the CA data from the client config.
	caFile, caData := config.CAFile, []byte{}
	if len(caFile) == 0 {
		caData = config.CAData
	}

	kubeconfig, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{"node-agent": {
			Server:                   config.Host,
			InsecureSkipTLSVerify:    config.Insecure,
			CertificateAuthority:     caFile,
			CertificateAuthorityData: caData,
		}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"node-agent": {
				ClientCertificateData: certData,
				ClientKeyData:         privateKeyData,
			}},
		Contexts: map[string]*clientcmdapi.Context{
			"node-agent": {
				Cluster:  "node-agent",
				AuthInfo: "node-agent",
			}},
		CurrentContext: "node-agent",
	})
	if err != nil {
		return err
	}

	return fs.WriteFile(nodeagentv1alpha1.KubeconfigFilePath, kubeconfig, 0600)
}
