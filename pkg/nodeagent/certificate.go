// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
)

const nodeAgentCSRPrefix = "node-agent-csr-"

var codec runtime.Codec

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(nodeagentconfigv1alpha1.AddToScheme(scheme))
	ser := jsonserializer.NewSerializerWithOptions(jsonserializer.DefaultMetaFactory, scheme, scheme, jsonserializer.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	versions := schema.GroupVersions([]schema.GroupVersion{nodeagentconfigv1alpha1.SchemeGroupVersion})
	codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)
}

// RequestAndStoreKubeconfig requests a certificate via CSR and stores the resulting kubeconfig on the disk.
func RequestAndStoreKubeconfig(ctx context.Context, log logr.Logger, fs afero.Afero, config *rest.Config, machineName string) error {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("unable to create a clientset from rest config: %w", err)
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

	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
		"node-agent",
		clientcmdv1.Cluster{
			Server:                   config.Host,
			InsecureSkipTLSVerify:    config.Insecure,
			CertificateAuthority:     caFile,
			CertificateAuthorityData: caData,
		},
		clientcmdv1.AuthInfo{
			ClientCertificateData: certData,
			ClientKeyData:         privateKeyData,
		},
	))
	if err != nil {
		return fmt.Errorf("unable to encode the gardener-node-agent kubeconfig: %w", err)
	}

	return fs.WriteFile(nodeagentconfigv1alpha1.KubeconfigFilePath, kubeconfig, 0600)
}

// GetAPIServerConfig reads the gardener-node-agent config file and returns the APIServer configuration.
func GetAPIServerConfig(fs afero.Afero) (*nodeagentconfigv1alpha1.APIServer, error) {
	nodeAgentConfigFile, err := fs.ReadFile(nodeagentconfigv1alpha1.ConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading gardener-node-agent config file: %w", err)
	}

	nodeAgentConfig := &nodeagentconfigv1alpha1.NodeAgentConfiguration{}
	if err = runtime.DecodeInto(codec, nodeAgentConfigFile, nodeAgentConfig); err != nil {
		return nil, fmt.Errorf("error decoding gardener-node-agent config: %w", err)
	}

	return &nodeAgentConfig.APIServer, nil
}
