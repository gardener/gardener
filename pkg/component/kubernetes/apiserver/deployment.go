// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	netutils "github.com/gardener/gardener/pkg/utils/net"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	secretNameServerCert             = "kube-apiserver"
	secretNameKubeAPIServerToKubelet = "kube-apiserver-kubelet"    // #nosec G101 -- No credential.
	secretNameKubeAggregator         = "kube-aggregator"           // #nosec G101 -- No credential.
	secretNameHTTPProxy              = "kube-apiserver-http-proxy" // #nosec G101 -- No credential.
	secretNameHAVPNSeedClient        = "vpn-seed-client"           // #nosec G101 -- No credential.

	// ContainerNameKubeAPIServer is the name of the kube-apiserver container.
	ContainerNameKubeAPIServer     = "kube-apiserver"
	containerNameVPNPathController = "vpn-path-controller"
	containerNameVPNSeedClient     = "vpn-client"

	volumeNameAuthenticationWebhookKubeconfig = "authentication-webhook-kubeconfig"
	volumeNameCA                              = "ca"
	volumeNameCAClient                        = "ca-client"
	volumeNameCAFrontProxy                    = "ca-front-proxy"
	volumeNameCAKubelet                       = "ca-kubelet"
	volumeNameCAVPN                           = "ca-vpn"
	volumeNameEgressSelector                  = "egress-selection-config"
	volumeNameHTTPProxy                       = "http-proxy"
	volumeNameKubeAPIServerToKubelet          = "kubelet-client"
	volumeNameKubeAggregator                  = "kube-aggregator"
	volumeNameOIDCCABundle                    = "oidc-cabundle"
	volumeNameServiceAccountKey               = "service-account-key"
	volumeNameServiceAccountKeyBundle         = "service-account-key-bundle"
	volumeNameStaticToken                     = "static-token"
	volumeNamePrefixTLSSNISecret              = "tls-sni-"
	volumeNameVPNSeedClient                   = "vpn-seed-client"
	volumeNameAPIServerAccess                 = "kube-api-access-gardener"
	volumeNameVPNSeedTLSAuth                  = "vpn-seed-tlsauth"
	volumeNameDevNetTun                       = "dev-net-tun"

	volumeMountPathAuthenticationWebhookKubeconfig = "/etc/kubernetes/webhook/authentication"
	volumeMountPathCA                              = "/srv/kubernetes/ca"
	volumeMountPathCAClient                        = "/srv/kubernetes/ca-client"
	volumeMountPathCAFrontProxy                    = "/srv/kubernetes/ca-front-proxy"
	volumeMountPathCAKubelet                       = "/srv/kubernetes/ca-kubelet"
	volumeMountPathCAVPN                           = "/srv/kubernetes/ca-vpn"
	volumeMountPathEgressSelector                  = "/etc/kubernetes/egress"
	volumeMountPathHTTPProxy                       = "/etc/srv/kubernetes/envoy"
	volumeMountPathKubeAPIServerToKubelet          = "/srv/kubernetes/apiserver-kubelet"
	volumeMountPathKubeAggregator                  = "/srv/kubernetes/aggregator"
	volumeMountPathServiceAccountKey               = "/srv/kubernetes/service-account-key"
	volumeMountPathServiceAccountKeyBundle         = "/srv/kubernetes/service-account-key-bundle"
	volumeMountPathStaticToken                     = "/srv/kubernetes/token"    // #nosec G101 -- No credential.
	volumeMountPathPrefixTLSSNISecret              = "/srv/kubernetes/tls-sni/" // #nosec G101 -- No credential.
	volumeMountPathVPNSeedClient                   = "/srv/secrets/vpn-client"
	volumeMountPathAPIServerAccess                 = "/var/run/secrets/kubernetes.io/serviceaccount"
	volumeMountPathVPNSeedTLSAuth                  = "/srv/secrets/tlsauth"
	volumeMountPathDevNetTun                       = "/dev/net/tun"
)

func (k *kubeAPIServer) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileDeployment(
	ctx context.Context,
	deployment *appsv1.Deployment,
	serviceAccount *corev1.ServiceAccount,
	configMapAuditPolicy *corev1.ConfigMap,
	configMapAuthenticationConfig *corev1.ConfigMap,
	configMapAuthorizationConfig *corev1.ConfigMap,
	configMapAdmissionConfigs *corev1.ConfigMap,
	secretAdmissionKubeconfigs *corev1.Secret,
	configMapEgressSelector *corev1.ConfigMap,
	secretETCDEncryptionConfiguration *corev1.Secret,
	secretOIDCCABundle *corev1.Secret,
	secretServiceAccountKey *corev1.Secret,
	secretStaticToken *corev1.Secret,
	secretServer *corev1.Secret,
	secretKubeletClient *corev1.Secret,
	secretKubeAggregator *corev1.Secret,
	secretHTTPProxy *corev1.Secret,
	secretHAVPNSeedClient *corev1.Secret,
	secretHAVPNSeedClientSeedTLSAuth *corev1.Secret,
	secretAuditWebhookKubeconfig *corev1.Secret,
	secretAuthenticationWebhookKubeconfig *corev1.Secret,
	secretAuthorizationWebhooksKubeconfigs *corev1.Secret,
	tlsSNISecrets []tlsSNISecret,
) error {
	var (
		maxSurge       = intstr.FromString("100%")
		maxUnavailable = intstr.FromInt32(0)
	)

	var healthCheckToken string
	if secretStaticToken != nil {
		staticToken, err := secrets.LoadStaticTokenFromCSV(SecretStaticTokenName, secretStaticToken.Data[secrets.DataKeyStaticTokenCSV])
		if err != nil {
			return err
		}

		token, err := staticToken.GetTokenForUsername(userNameHealthCheck)
		if err != nil {
			return err
		}

		healthCheckToken = token.Token
	}

	secretCACluster, found := k.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	secretCAClient, found := k.secretsManager.Get(v1beta1constants.SecretNameCAClient)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAClient)
	}

	secretCAFrontProxy, found := k.secretsManager.Get(v1beta1constants.SecretNameCAFrontProxy)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAFrontProxy)
	}

	secretCAETCD, found := k.secretsManager.Get(v1beta1constants.SecretNameCAETCD)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCD)
	}

	secretETCDClient, found := k.secretsManager.Get(etcd.SecretNameClient)
	if !found {
		return fmt.Errorf("secret %q not found", etcd.SecretNameClient)
	}

	secretServiceAccountKeyBundle, found := k.secretsManager.Get(v1beta1constants.SecretNameServiceAccountKey, secretsmanager.Bundle)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameServiceAccountKey)
	}

	if err := netutils.CheckDualStackForKubeComponents(k.values.ServiceNetworkCIDRs, "service"); err != nil {
		return err
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(GetLabels(), map[string]string{
			resourcesv1alpha1.HighAvailabilityConfigType:                        resourcesv1alpha1.HighAvailabilityConfigTypeServer,
			v1beta1constants.LabelExtensionProviderMutatedByControlplaneWebhook: "true",
		})
		deployment.Spec = appsv1.DeploymentSpec{
			MinReadySeconds:      30,
			RevisionHistoryLimit: ptr.To[int32](2),
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
					Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:                                                      v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPublicNetworks:                                           v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks:                                          v1beta1constants.LabelNetworkPolicyAllowed,
						"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicyWebhookTargets: v1beta1constants.LabelNetworkPolicyAllowed,
						"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicyExtensionsNamespaceAlias + "-" + v1beta1constants.LabelNetworkPolicyWebhookTargets: v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel(k.values.NamePrefix+etcdconstants.ServiceName(v1beta1constants.ETCDRoleMain), etcdconstants.PortEtcdClient):                      v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel(k.values.NamePrefix+etcdconstants.ServiceName(v1beta1constants.ETCDRoleEvents), etcdconstants.PortEtcdClient):                    v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken:  ptr.To(false),
					PriorityClassName:             k.values.PriorityClassName,
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 corev1.DefaultSchedulerName,
					TerminationGracePeriodSeconds: ptr.To[int64](30),
					SecurityContext: &corev1.PodSecurityContext{
						// use the nonroot user from a distroless container
						// https://github.com/GoogleContainerTools/distroless/blob/1a8918fcaa7313fd02ae08089a57a701faea999c/base/base.bzl#L8
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](65532),
						RunAsGroup:   ptr.To[int64](65532),
						FSGroup:      ptr.To[int64](65532),
					},
					Containers: []corev1.Container{{
						Name:                     ContainerNameKubeAPIServer,
						Image:                    k.values.Images.KubeAPIServer,
						ImagePullPolicy:          corev1.PullIfNotPresent,
						Command:                  []string{"/usr/local/bin/kube-apiserver"},
						Args:                     k.computeKubeAPIServerArgs(),
						TerminationMessagePath:   corev1.TerminationMessagePathDefault,
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						Ports: []corev1.ContainerPort{{
							Name:          "https",
							ContainerPort: kubeapiserverconstants.Port,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources: k.values.Autoscaling.APIServerResources,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/livez",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt32(kubeapiserverconstants.Port),
									HTTPHeaders: []corev1.HTTPHeader{{
										Name:  "Authorization",
										Value: "Bearer " + healthCheckToken,
									}},
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/readyz",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt32(kubeapiserverconstants.Port),
									HTTPHeaders: []corev1.HTTPHeader{{
										Name:  "Authorization",
										Value: "Bearer " + healthCheckToken,
									}},
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volumeNameCA,
								MountPath: volumeMountPathCA,
							},
							{
								Name:      volumeNameCAClient,
								MountPath: volumeMountPathCAClient,
							},
							{
								Name:      volumeNameCAFrontProxy,
								MountPath: volumeMountPathCAFrontProxy,
							},
							{
								Name:      volumeNameServiceAccountKey,
								MountPath: volumeMountPathServiceAccountKey,
							},
							{
								Name:      volumeNameServiceAccountKeyBundle,
								MountPath: volumeMountPathServiceAccountKeyBundle,
							},
							{
								Name:      volumeNameStaticToken,
								MountPath: volumeMountPathStaticToken,
							},
							{
								Name:      volumeNameKubeAggregator,
								MountPath: volumeMountPathKubeAggregator,
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: volumeNameCA,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretCACluster.Name,
								},
							},
						},
						{
							Name: volumeNameCAClient,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretCAClient.Name,
								},
							},
						},
						{
							Name: volumeNameCAFrontProxy,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretCAFrontProxy.Name,
								},
							},
						},
						{
							Name: volumeNameServiceAccountKey,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretServiceAccountKey.Name,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						{
							Name: volumeNameServiceAccountKeyBundle,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretServiceAccountKeyBundle.Name,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						{
							Name: volumeNameStaticToken,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretStaticToken.Name,
								},
							},
						},
						{
							Name: volumeNameKubeAggregator,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretKubeAggregator.Name,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
					},
				},
			},
		}

		apiserver.InjectDefaultSettings(deployment, k.values.NamePrefix, k.values.Values, secretCAETCD, secretETCDClient, secretServer)
		apiserver.InjectAuditSettings(deployment, configMapAuditPolicy, secretAuditWebhookKubeconfig, k.values.Audit)
		apiserver.InjectAdmissionSettings(deployment, configMapAdmissionConfigs, secretAdmissionKubeconfigs, k.values.Values)
		apiserver.InjectEncryptionSettings(deployment, secretETCDEncryptionConfiguration)
		k.handleSNISettings(deployment)
		k.handleTLSSNISettings(deployment, tlsSNISecrets)
		k.handleServiceAccountSigningKeySettings(deployment)
		k.handleAuthenticationSettings(deployment, configMapAuthenticationConfig, secretOIDCCABundle)
		k.handleAuthenticationWebhookSettings(deployment, secretAuthenticationWebhookKubeconfig)
		k.handleAuthorizationSettings(deployment, configMapAuthorizationConfig, secretAuthorizationWebhooksKubeconfigs)
		if err := k.handleVPNSettings(deployment, serviceAccount, configMapEgressSelector, secretHTTPProxy, secretHAVPNSeedClient, secretHAVPNSeedClientSeedTLSAuth); err != nil {
			return err
		}
		if err := k.handleKubeletSettings(deployment, secretKubeletClient); err != nil {
			return err
		}

		utilruntime.Must(references.InjectAnnotations(deployment))
		return nil
	})
	return err
}

func (k *kubeAPIServer) computeKubeAPIServerArgs() []string {
	var out []string

	out = append(out, "--anonymous-auth="+strconv.FormatBool(k.values.AnonymousAuthenticationEnabled))

	if len(k.values.APIAudiences) > 0 {
		out = append(out, "--api-audiences="+strings.Join(k.values.APIAudiences, ","))
	}

	out = append(out, fmt.Sprintf("--client-ca-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyCertificateBundle))
	out = append(out, "--enable-aggregator-routing=true")
	out = append(out, "--enable-bootstrap-token-auth=true")
	out = append(out, "--etcd-servers-overrides="+k.etcdServersOverrides())
	out = append(out, "--external-hostname="+k.values.ExternalHostname)

	if k.values.DefaultNotReadyTolerationSeconds != nil {
		out = append(out, fmt.Sprintf("--default-not-ready-toleration-seconds=%d", *k.values.DefaultNotReadyTolerationSeconds))
	}
	if k.values.DefaultUnreachableTolerationSeconds != nil {
		out = append(out, fmt.Sprintf("--default-unreachable-toleration-seconds=%d", *k.values.DefaultUnreachableTolerationSeconds))
	}

	if k.values.EventTTL != nil {
		out = append(out, fmt.Sprintf("--event-ttl=%s", k.values.EventTTL.Duration))
	}

	out = append(out, fmt.Sprintf("--proxy-client-cert-file=%s/%s", volumeMountPathKubeAggregator, secrets.DataKeyCertificate))
	out = append(out, fmt.Sprintf("--proxy-client-key-file=%s/%s", volumeMountPathKubeAggregator, secrets.DataKeyPrivateKey))
	out = append(out, fmt.Sprintf("--requestheader-client-ca-file=%s/%s", volumeMountPathCAFrontProxy, secrets.DataKeyCertificateBundle))
	out = append(out, "--requestheader-extra-headers-prefix=X-Remote-Extra-")
	out = append(out, "--requestheader-group-headers=X-Remote-Group")
	out = append(out, "--requestheader-username-headers=X-Remote-User")

	if k.values.IsWorkerless {
		disableAPIs := map[string]bool{
			"autoscaling/v2":                 false,
			"batch/v1":                       false,
			"apps/v1":                        false,
			"policy/v1/poddisruptionbudgets": false,
			"storage.k8s.io/v1/csidrivers":   false,
			"storage.k8s.io/v1/csinodes":     false,
		}

		// Allow users to explicitly enable disabled APIs via RuntimeConfig.
		k.values.RuntimeConfig = utils.MergeStringMaps(disableAPIs, k.values.RuntimeConfig)
	}

	if k.values.RuntimeConfig != nil {
		out = append(out, kubernetesutils.MapStringBoolToCommandLineParameter(k.values.RuntimeConfig, "--runtime-config="))
	}

	out = append(out, "--service-account-issuer="+k.values.ServiceAccount.Issuer)
	for _, issuer := range k.values.ServiceAccount.AcceptedIssuers {
		out = append(out, fmt.Sprintf("--service-account-issuer=%s", issuer))
	}
	if k.values.ServiceAccount.JWKSURI != nil {
		out = append(out, fmt.Sprintf("--service-account-jwks-uri=%s", *k.values.ServiceAccount.JWKSURI))
	}
	if k.values.ServiceAccount.ExtendTokenExpiration != nil {
		out = append(out, fmt.Sprintf("--service-account-extend-token-expiration=%s", strconv.FormatBool(*k.values.ServiceAccount.ExtendTokenExpiration)))
	}
	if k.values.ServiceAccount.MaxTokenExpiration != nil {
		out = append(out, fmt.Sprintf("--service-account-max-token-expiration=%s", k.values.ServiceAccount.MaxTokenExpiration.Duration))
	}

	out = append(out, fmt.Sprintf("--service-cluster-ip-range=%s", netutils.JoinByComma(k.values.ServiceNetworkCIDRs)))
	out = append(out, fmt.Sprintf("--secure-port=%d", kubeapiserverconstants.Port))
	out = append(out, fmt.Sprintf("--token-auth-file=%s/%s", volumeMountPathStaticToken, secrets.DataKeyStaticTokenCSV))

	return out
}

func (k *kubeAPIServer) etcdServersOverrides() string {
	addGroupResourceIfNotPresent := func(groupResources []schema.GroupResource, groupResource schema.GroupResource) []schema.GroupResource {
		for _, resource := range groupResources {
			if resource.Group == groupResource.Group && resource.Resource == groupResource.Resource {
				return groupResources
			}
		}
		return append([]schema.GroupResource{groupResource}, groupResources...)
	}

	var overrides []string
	for _, resource := range addGroupResourceIfNotPresent(k.values.ResourcesToStoreInETCDEvents, schema.GroupResource{Resource: "events"}) {
		overrides = append(overrides, fmt.Sprintf("%s/%s#https://%s%s:%d", resource.Group, resource.Resource, k.values.NamePrefix, etcdconstants.ServiceName(v1beta1constants.ETCDRoleEvents), etcdconstants.PortEtcdClient))
	}
	return strings.Join(overrides, ",")
}

func (k *kubeAPIServer) handleSNISettings(deployment *appsv1.Deployment) {
	if !k.values.SNI.Enabled {
		return
	}

	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--advertise-address=%s", k.values.SNI.AdvertiseAddress))
}

func (k *kubeAPIServer) handleTLSSNISettings(deployment *appsv1.Deployment, tlsSNISecrets []tlsSNISecret) {
	for i, sni := range tlsSNISecrets {
		var (
			volumeName      = fmt.Sprintf("%s%d", volumeNamePrefixTLSSNISecret, i)
			volumeMountPath = fmt.Sprintf("%s%d", volumeMountPathPrefixTLSSNISecret, i)
			flag            = fmt.Sprintf("--tls-sni-cert-key=%s/tls.crt,%s/tls.key", volumeMountPath, volumeMountPath)
		)

		if len(sni.domainPatterns) > 0 {
			flag += ":" + strings.Join(sni.domainPatterns, ",")
		}

		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, flag)
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: volumeMountPath,
			ReadOnly:  true,
		})
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  sni.secretName,
					DefaultMode: ptr.To[int32](0640),
				},
			},
		})
	}
}

func (k *kubeAPIServer) handleVPNSettings(
	deployment *appsv1.Deployment,
	serviceAccount *corev1.ServiceAccount,
	configMapEgressSelector *corev1.ConfigMap,
	secretHTTPProxy *corev1.Secret,
	secretHAVPNSeedClient *corev1.Secret,
	secretHAVPNSeedClientSeedTLSAuth *corev1.Secret,
) error {
	if !k.values.VPN.Enabled {
		return nil
	}

	secretCAVPN, found := k.secretsManager.Get(v1beta1constants.SecretNameCAVPN)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAVPN)
	}

	if k.values.VPN.HighAvailabilityEnabled {
		k.handleVPNSettingsHA(deployment, serviceAccount, secretCAVPN, secretHAVPNSeedClient, secretHAVPNSeedClientSeedTLSAuth)
	} else {
		k.handleVPNSettingsNonHA(deployment, secretCAVPN, secretHTTPProxy, configMapEgressSelector)
	}

	return nil
}

func (k *kubeAPIServer) handleVPNSettingsNonHA(
	deployment *appsv1.Deployment,
	secretCAVPN *corev1.Secret,
	secretHTTPProxy *corev1.Secret,
	configMapEgressSelector *corev1.ConfigMap,
) {
	deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
		gardenerutils.NetworkPolicyLabel(vpnseedserver.ServiceName, vpnseedserver.EnvoyPort): v1beta1constants.LabelNetworkPolicyAllowed,
	})
	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--egress-selector-config-file=%s/%s", volumeMountPathEgressSelector, configMapEgressSelectorDataKey))
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
		{
			Name:      volumeNameCAVPN,
			MountPath: volumeMountPathCAVPN,
		},
		{
			Name:      volumeNameHTTPProxy,
			MountPath: volumeMountPathHTTPProxy,
		},
		{
			Name:      volumeNameEgressSelector,
			MountPath: volumeMountPathEgressSelector,
		},
	}...)
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
		{
			Name: volumeNameCAVPN,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretCAVPN.Name,
				},
			},
		},
		{
			Name: volumeNameHTTPProxy,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretHTTPProxy.Name,
					DefaultMode: ptr.To[int32](0640),
				},
			},
		},
		{
			Name: volumeNameEgressSelector,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapEgressSelector.Name,
					},
				},
			},
		},
	}...)
}

func (k *kubeAPIServer) handleVPNSettingsHA(
	deployment *appsv1.Deployment,
	serviceAccount *corev1.ServiceAccount,
	secretCAVPN *corev1.Secret,
	secretHAVPNSeedClient *corev1.Secret,
	secretHAVPNSeedClientSeedTLSAuth *corev1.Secret,
) {
	for i := 0; i < k.values.VPN.HighAvailabilityNumberOfSeedServers; i++ {
		serviceName := fmt.Sprintf("%s-%d", vpnseedserver.ServiceName, i)

		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			gardenerutils.NetworkPolicyLabel(serviceName, vpnseedserver.OpenVPNPort): v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	deployment.Spec.Template.Spec.ServiceAccountName = serviceAccount.Name
	deployment.Spec.Template.Labels[v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer] = v1beta1constants.LabelNetworkPolicyAllowed

	// vpn-path-controller uses ping to probe clients. By default, the kernel allows no groups to do unprivileged pings.
	// Configuring this parameter allows group 0 to do unprivileged pings.
	deployment.Spec.Template.Spec.SecurityContext.Sysctls = append(deployment.Spec.Template.Spec.SecurityContext.Sysctls,
		corev1.Sysctl{
			Name:  "net.ipv4.ping_group_range",
			Value: "0 0",
		})

	for i := 0; i < k.values.VPN.HighAvailabilityNumberOfSeedServers; i++ {
		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *k.vpnSeedClientContainer(i))
	}
	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *k.vpnSeedPathControllerContainer())
	deployment.Spec.Template.Spec.InitContainers = append(deployment.Spec.Template.Spec.InitContainers, *k.vpnSeedClientInitContainer())

	hostPathCharDev := corev1.HostPathCharDev
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
		{
			Name: volumeNameAPIServerAccess,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0640),
					Sources: []corev1.VolumeProjection{
						{
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								ExpirationSeconds: ptr.To(int64(60 * 60 * 12)),
								Path:              "token",
							},
						},
						{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "kube-root-ca.crt",
								},
								Items: []corev1.KeyToPath{{
									Key:  "ca.crt",
									Path: "ca.crt",
								}},
							},
						},
						{
							DownwardAPI: &corev1.DownwardAPIProjection{
								Items: []corev1.DownwardAPIVolumeFile{{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "metadata.namespace",
									},
									Path: "namespace",
								}},
							},
						},
					},
				},
			},
		},
		{
			Name: volumeNameVPNSeedClient,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0640),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretCAVPN.Name,
								},
								Items: []corev1.KeyToPath{{
									Key:  secrets.DataKeyCertificateBundle,
									Path: secrets.DataKeyCertificateCA,
								}},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretHAVPNSeedClient.Name,
								},
								Items: []corev1.KeyToPath{
									{
										Key:  secrets.DataKeyCertificate,
										Path: secrets.DataKeyCertificate,
									},
									{
										Key:  secrets.DataKeyPrivateKey,
										Path: secrets.DataKeyPrivateKey,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: volumeNameVPNSeedTLSAuth,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretHAVPNSeedClientSeedTLSAuth.Name,
					DefaultMode: ptr.To[int32](0640),
				},
			},
		},
		{
			Name: volumeNameDevNetTun,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volumeMountPathDevNetTun,
					Type: &hostPathCharDev,
				},
			},
		},
	}...)
}

func (k *kubeAPIServer) vpnSeedClientInitContainer() *corev1.Container {
	container := k.vpnSeedClientContainer(0)
	container.Name = "vpn-client-init"
	container.Command = []string{"/bin/vpn-client", "setup"}
	container.Env = append(container.Env, []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}...)

	if !k.values.VPN.DisableNewVPN {
		// may need to enable IPv6 in pod network (e.g. for GKE clusters)
		container.SecurityContext.Privileged = ptr.To(true)
	}

	if k.values.VPN.DisableNewVPN {
		container.Command = nil
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name:  "EXIT_AFTER_CONFIGURING_KERNEL_SETTINGS",
				Value: "true",
			},
			corev1.EnvVar{
				Name:  "CONFIGURE_BONDING",
				Value: "true",
			})
	}

	container.LivenessProbe = nil
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      volumeNameAPIServerAccess,
		MountPath: volumeMountPathAPIServerAccess,
		ReadOnly:  true,
	})
	return container
}

func (k *kubeAPIServer) vpnSeedClientContainer(index int) *corev1.Container {
	var serviceCIDRs, podCIDRs, nodeCIDRs []string
	nodeCIDR := ""
	if len(k.values.VPN.NodeNetworkCIDRs) > 0 {
		nodeCIDR = k.values.VPN.NodeNetworkCIDRs[0].String()
		for _, v := range k.values.VPN.NodeNetworkCIDRs {
			nodeCIDRs = append(nodeCIDRs, v.String())
		}
	}

	for _, v := range k.values.ServiceNetworkCIDRs {
		serviceCIDRs = append(serviceCIDRs, v.String())
	}

	for _, v := range k.values.VPN.PodNetworkCIDRs {
		podCIDRs = append(podCIDRs, v.String())
	}

	container := &corev1.Container{
		Name:            fmt.Sprintf("%s-%d", containerNameVPNSeedClient, index),
		Image:           k.values.Images.VPNClient,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			{
				Name:  "ENDPOINT",
				Value: fmt.Sprintf("vpn-seed-server-%d", index),
			},
			{
				Name:  "SERVICE_NETWORK",
				Value: k.values.ServiceNetworkCIDRs[0].String(),
			},
			{
				Name:  "POD_NETWORK",
				Value: k.values.VPN.PodNetworkCIDRs[0].String(),
			},
			{
				Name:  "NODE_NETWORK",
				Value: nodeCIDR,
			},
			{
				Name:  "SERVICE_NETWORKS",
				Value: strings.Join(serviceCIDRs, ","),
			},
			{
				Name:  "POD_NETWORKS",
				Value: strings.Join(podCIDRs, ","),
			},
			{
				Name:  "NODE_NETWORKS",
				Value: strings.Join(nodeCIDRs, ","),
			},
			{
				Name:  "VPN_SERVER_INDEX",
				Value: strconv.Itoa(index),
			},
			{
				Name:  "IS_HA",
				Value: "true",
			},
			{
				Name:  "HA_VPN_SERVERS",
				Value: strconv.Itoa(k.values.VPN.HighAvailabilityNumberOfSeedServers),
			},
			{
				Name:  "HA_VPN_CLIENTS",
				Value: strconv.Itoa(k.values.VPN.HighAvailabilityNumberOfShootClients),
			},
			{
				Name:  "OPENVPN_PORT",
				Value: strconv.Itoa(vpnseedserver.OpenVPNPort),
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("10Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot: ptr.To(false),
			RunAsUser:    ptr.To[int64](0),
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
		},
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeNameVPNSeedClient,
				MountPath: volumeMountPathVPNSeedClient,
			},
			{
				Name:      volumeNameVPNSeedTLSAuth,
				MountPath: volumeMountPathVPNSeedTLSAuth,
			},
			{
				Name:      volumeNameDevNetTun,
				MountPath: volumeMountPathDevNetTun,
			},
		},
	}

	if len(k.values.VPN.IPFamilies) > 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			// IP_FAMILIES of seed
			Name:  "IP_FAMILIES",
			Value: string(k.values.VPN.IPFamilies[0]),
		})
	}

	if k.values.VPN.DisableNewVPN {
		container.Command = nil
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "DO_NOT_CONFIGURE_KERNEL_SETTINGS",
			Value: "true",
		})
	}

	return container
}

func (k *kubeAPIServer) vpnSeedPathControllerContainer() *corev1.Container {
	var serviceCIDRs, podCIDRs, nodeCIDRs []string
	nodeCIDR := ""
	if len(k.values.VPN.NodeNetworkCIDRs) > 0 {
		nodeCIDR = k.values.VPN.NodeNetworkCIDRs[0].String()
		for _, v := range k.values.VPN.NodeNetworkCIDRs {
			nodeCIDRs = append(nodeCIDRs, v.String())
		}
	}

	for _, v := range k.values.ServiceNetworkCIDRs {
		serviceCIDRs = append(serviceCIDRs, v.String())
	}

	for _, v := range k.values.VPN.PodNetworkCIDRs {
		podCIDRs = append(podCIDRs, v.String())
	}

	container := &corev1.Container{
		Name:            containerNameVPNPathController,
		Image:           k.values.Images.VPNClient,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/vpn-client", "path-controller"},
		Env: []corev1.EnvVar{
			{
				Name:  "SERVICE_NETWORK",
				Value: k.values.ServiceNetworkCIDRs[0].String(),
			},
			{
				Name:  "POD_NETWORK",
				Value: k.values.VPN.PodNetworkCIDRs[0].String(),
			},
			{
				Name:  "NODE_NETWORK",
				Value: nodeCIDR,
			},
			{
				Name:  "SERVICE_NETWORKS",
				Value: strings.Join(serviceCIDRs, ","),
			},
			{
				Name:  "POD_NETWORKS",
				Value: strings.Join(podCIDRs, ","),
			},
			{
				Name:  "NODE_NETWORKS",
				Value: strings.Join(nodeCIDRs, ","),
			},
			{
				Name:  "IS_HA",
				Value: "true",
			},
			{
				Name:  "HA_VPN_CLIENTS",
				Value: strconv.Itoa(k.values.VPN.HighAvailabilityNumberOfShootClients),
			},
			{
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("10Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot: ptr.To(false),
			RunAsUser:    ptr.To[int64](0),
			// group needs to be set to a concrete value to allow unprivileged ping socket when configuring sysctl net.ipv4.ping_group_range
			RunAsGroup: ptr.To[int64](0),
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
		},
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
	}

	if k.values.VPN.DisableNewVPN {
		container.Command = nil
		container.Args = []string{"/path-controller.sh"}
	}

	return container
}

func (k *kubeAPIServer) handleServiceAccountSigningKeySettings(deployment *appsv1.Deployment) {
	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--service-account-signing-key-file=%s/%s", volumeMountPathServiceAccountKey, secrets.DataKeyRSAPrivateKey))
	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--service-account-key-file=%s/%s", volumeMountPathServiceAccountKeyBundle, secrets.DataKeyPrivateKeyBundle))
}

func (k *kubeAPIServer) handleKubeletSettings(deployment *appsv1.Deployment, secretKubeletClient *corev1.Secret) error {
	if k.values.IsWorkerless {
		return nil
	}

	secretCAKubelet, found := k.secretsManager.Get(v1beta1constants.SecretNameCAKubelet)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAKubelet)
	}

	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args,
		"--allow-privileged=true",
		"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
		fmt.Sprintf("--kubelet-certificate-authority=%s/%s", volumeMountPathCAKubelet, secrets.DataKeyCertificateBundle),
		fmt.Sprintf("--kubelet-client-certificate=%s/%s", volumeMountPathKubeAPIServerToKubelet, secrets.DataKeyCertificate),
		fmt.Sprintf("--kubelet-client-key=%s/%s", volumeMountPathKubeAPIServerToKubelet, secrets.DataKeyPrivateKey),
	)
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
		{
			Name:      volumeNameCAKubelet,
			MountPath: volumeMountPathCAKubelet,
		},
		{
			Name:      volumeNameKubeAPIServerToKubelet,
			MountPath: volumeMountPathKubeAPIServerToKubelet,
		},
	}...)
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
		{
			Name: volumeNameCAKubelet,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretCAKubelet.Name,
				},
			},
		},
		{
			Name: volumeNameKubeAPIServerToKubelet,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretKubeletClient.Name,
					DefaultMode: ptr.To[int32](0640),
				},
			},
		},
	}...)

	return nil
}

func (k *kubeAPIServer) handleAuthenticationWebhookSettings(deployment *appsv1.Deployment, secretWebhookKubeconfig *corev1.Secret) {
	if k.values.AuthenticationWebhook == nil {
		return
	}

	if len(k.values.AuthenticationWebhook.Kubeconfig) > 0 {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authentication-token-webhook-config-file=%s/%s", volumeMountPathAuthenticationWebhookKubeconfig, apiserver.SecretWebhookKubeconfigDataKey))
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeNameAuthenticationWebhookKubeconfig,
			MountPath: volumeMountPathAuthenticationWebhookKubeconfig,
			ReadOnly:  true,
		})
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeNameAuthenticationWebhookKubeconfig,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretWebhookKubeconfig.Name,
				},
			},
		})
	}

	if v := k.values.AuthenticationWebhook.CacheTTL; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authentication-token-webhook-cache-ttl=%s", v.String()))
	}

	if v := k.values.AuthenticationWebhook.Version; v != nil {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--authentication-token-webhook-version=%s", *v))
	}
}
