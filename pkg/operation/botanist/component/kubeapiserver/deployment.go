// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserver

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/gardener/gardener-resource-manager/pkg/controller/garbagecollector/references"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
)

const (
	// SecretNameBasicAuth is the name of the secret containing basic authentication credentials for the kube-apiserver.
	SecretNameBasicAuth = "kube-apiserver-basic-auth"
	// SecretNameEtcdEncryption is the name of the secret which contains the EncryptionConfiguration. The
	// EncryptionConfiguration contains a key which the kube-apiserver uses for encrypting selected etcd content.
	SecretNameEtcdEncryption = "etcd-encryption-secret"
	// SecretNameKubeAggregator is the name of the secret for the kube-aggregator when talking to the kube-apiserver.
	SecretNameKubeAggregator = "kube-aggregator"
	// SecretNameKubeAPIServerToKubelet is the name of the secret for the kube-apiserver credentials when talking to
	// kubelets.
	SecretNameKubeAPIServerToKubelet = "kube-apiserver-kubelet"
	// SecretNameServer is the name of the secret for the kube-apiserver server certificates.
	SecretNameServer = "kube-apiserver"
	// SecretNameStaticToken is the name of the secret containing static tokens for the kube-apiserver.
	SecretNameStaticToken = "static-token"
	// SecretNameVPNSeed is the name of the secret containing the certificates for the vpn-seed.
	SecretNameVPNSeed = "vpn-seed"
	// SecretNameVPNSeedTLSAuth is the name of the secret containing the TLS auth for the vpn-seed.
	SecretNameVPNSeedTLSAuth = "vpn-seed-tlsauth"

	containerNameKubeAPIServer            = "kube-apiserver"
	containerNameVPNSeed                  = "vpn-seed"
	containerNameAPIServerProxyPodMutator = "apiserver-proxy-pod-mutator"

	volumeMountPathAdmissionConfiguration = "/etc/kubernetes/admission"
	volumeMountPathHTTPProxy              = "/etc/srv/kubernetes/envoy"
)

func (k *kubeAPIServer) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	var (
		maxSurge       = intstr.FromString("25%")
		maxUnavailable = intstr.FromInt(0)
	)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), deployment, func() error {
		deployment.Labels = GetLabels()
		if k.values.SNI.Enabled {
			deployment.Labels[v1beta1constants.LabelAPIServerExposure] = v1beta1constants.LabelAPIServerExposureGardenerManaged
		}

		deployment.Spec = appsv1.DeploymentSpec{
			MinReadySeconds:      30,
			RevisionHistoryLimit: pointer.Int32(2),
			Replicas:             k.values.Autoscaling.Replicas,
			Selector:             &metav1.LabelSelector{MatchLabels: getLabels()},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: k.computePodAnnotations(),
					Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
						v1beta1constants.DeprecatedGardenRole:                v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToShootNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyFromPrometheus:    v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
			},
		}

		utilruntime.Must(references.InjectAnnotations(deployment))
		return nil
	})
	return err
}

func (k *kubeAPIServer) computePodAnnotations() map[string]string {
	out := make(map[string]string)

	for _, s := range k.secrets.all() {
		if s.Secret != nil && s.Name != "" && s.Checksum != "" {
			out["checksum/secret-"+s.Name] = s.Checksum
		}
	}

	return out
}
