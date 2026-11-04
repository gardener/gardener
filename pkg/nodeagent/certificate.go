// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"k8s.io/client-go/tools/clientcmd"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	nodeagenthelper "github.com/gardener/gardener/pkg/api/config/nodeagent/v1alpha1/helper"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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

// EnsureServerInKubeconfig ensures that the kubeconfig stored on the disk uses the given API server address. The
// kubeconfig is only written once when requesting the client certificate, but the API server address in the
// gardener-node-agent config might change afterward. For example, on the first control plane node of a self-hosted
// shoot, gardener-node-agent talks to the API server via localhost while bootstrapping the control plane and switches to
// the internal domain afterward.
func EnsureServerInKubeconfig(log logr.Logger, fs afero.Afero, server string) error {
	kubeconfigRaw, err := fs.ReadFile(nodeagentconfigv1alpha1.KubeconfigFilePath)
	if err != nil {
		return fmt.Errorf("failed reading kubeconfig file %q: %w", nodeagentconfigv1alpha1.KubeconfigFilePath, err)
	}

	kubeconfig, err := clientcmd.Load(kubeconfigRaw)
	if err != nil {
		return fmt.Errorf("failed loading kubeconfig file %q: %w", nodeagentconfigv1alpha1.KubeconfigFilePath, err)
	}

	var changed bool
	for name, cluster := range kubeconfig.Clusters {
		if cluster.Server != server {
			log.Info("Updating API server address in kubeconfig", "cluster", name, "oldServer", cluster.Server, "newServer", server)
			cluster.Server = server
			changed = true
		}
	}

	if !changed {
		return nil
	}

	kubeconfigRaw, err = clientcmd.Write(*kubeconfig)
	if err != nil {
		return fmt.Errorf("failed encoding kubeconfig: %w", err)
	}

	return fs.WriteFile(nodeagentconfigv1alpha1.KubeconfigFilePath, kubeconfigRaw, 0600)
}

// GetAPIServerConfig reads the gardener-node-agent config file and returns the APIServer configuration.
func GetAPIServerConfig(fs afero.Afero, configDir string) (*nodeagentconfigv1alpha1.APIServer, error) {
	nodeAgentConfigFile, err := fs.ReadFile(nodeagenthelper.GetConfigFilePath(configDir))
	if err != nil {
		return nil, fmt.Errorf("error reading gardener-node-agent config file: %w", err)
	}

	nodeAgentConfig := &nodeagentconfigv1alpha1.NodeAgentConfiguration{}
	if err = runtime.DecodeInto(codec, nodeAgentConfigFile, nodeAgentConfig); err != nil {
		return nil, fmt.Errorf("error decoding gardener-node-agent config: %w", err)
	}

	return &nodeAgentConfig.APIServer, nil
}
