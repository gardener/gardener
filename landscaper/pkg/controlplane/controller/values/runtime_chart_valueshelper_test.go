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

package values

import (
	"encoding/json"
	"fmt"
	"time"

	importsv1alpha1 "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1"
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuntimeChartValuesHelper", func() {
	var (
		err             error
		expectedValues  func(bool, bool) map[string]interface{}
		getValuesHelper func(bool, bool) RuntimeChartValuesHelper

		clusterIP       = "10.0.1.1"
		clusterIdentity = "landscape-one"
		// apiServerCACert = "caCertCM"
		// admissionControllerCACert = "admissionCertCM"
		rbac = importsv1alpha1.Rbac{
			SeedAuthorizer: &importsv1alpha1.SeedAuthorizer{Enabled: pointer.Bool(true)},
		}

		kubeconfigAPIServer           = "kubecfg-apiserver"
		kubeconfigControllerManager   = "kubecfg-gcm"
		kubeconfigScheduler           = "kubecfg-scheduler"
		kubeconfigAdmissionController = "kubecfg-admission"

		defaultCACrt  = "caCert"
		defaultCAKey  = "caCertKey"
		defaultTLSCrt = "tlsCert"
		defaultTLSKey = "tlsKey"

		etcdUrl        = "etcd.svc.local:2273"
		etcdCACrt      = "caCertEtcd"
		etcdClientCert = "etcdClientCert"
		etcdClientKey  = "etcdClientKey"

		testTargetConfigBytes []byte
		testTargetConfig      = &landscaperv1alpha1.KubernetesClusterTargetConfig{
			Kubeconfig: `apiVersion: landscaper.gardener.cloud/v1alpha1
kind: Target
spec:
type: landscaper.gardener.cloud/kubernetes-cluster
config:
  kubeconfig: |
	---
	apiVersion: v1
	clusters:
	  - cluster:
		  certificate-authority-data: fff
		  server: https://m
		name: sdf
	contexts:
	  - context:
		  cluster: v
		  user: d
		name: b
	current-context: shoot--garden-ls
	kind: Config
	preferences: {}
	users:
	  - name: abc
		user:
		  token: abc`,
		}

		admissionComponentConfig = &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
			Server: admissioncontrollerconfigv1alpha1.ServerConfiguration{
				ResourceAdmissionConfiguration: &admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration{
					Limits: []admissioncontrollerconfigv1alpha1.ResourceLimit{
						{
							Size: resource.MustParse("1"),
						},
					},
				},
			},
		}

		// minimal configuration to see if it gets marshalled
		gcmComponentConfig = controllermanagerconfigv1alpha1.ControllerManagerConfiguration{
			Controllers: controllermanagerconfigv1alpha1.ControllerManagerControllerConfiguration{
				Bastion: &controllermanagerconfigv1alpha1.BastionControllerConfiguration{
					ConcurrentSyncs: 10,
				},
			},
		}

		schedulerComponentConfig = schedulerconfigv1alpha1.SchedulerConfiguration{
			Schedulers: schedulerconfigv1alpha1.SchedulerControllerConfiguration{
				Shoot: &schedulerconfigv1alpha1.ShootSchedulerConfiguration{ConcurrentSyncs: 20},
			},
			FeatureGates: map[string]bool{
				"gago-feature": true,
			},
		}

		apiserverConfigurationOrig = importsv1alpha1.GardenerAPIServer{
			DeploymentConfiguration: &importsv1alpha1.APIServerDeploymentConfiguration{
				CommonDeploymentConfiguration: importsv1alpha1.CommonDeploymentConfiguration{
					ReplicaCount:       pointer.Int32(1),
					ServiceAccountName: pointer.String("sx"),
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"cpu": resource.MustParse("2"),
						},
						Requests: corev1.ResourceList{
							"memory": resource.MustParse("3Gi"),
						},
					},
					PodLabels:      map[string]string{"foo": "bar"},
					PodAnnotations: map[string]string{"foo": "annotation"},
					VPA:            pointer.Bool(true),
				},
				LivenessProbe: &corev1.Probe{
					InitialDelaySeconds: 5,
					TimeoutSeconds:      10,
					PeriodSeconds:       15,
				},
				ReadinessProbe: &corev1.Probe{
					InitialDelaySeconds: 5,
					TimeoutSeconds:      10,
					PeriodSeconds:       15,
				},
				MinReadySeconds: pointer.Int32(1),
				Hvpa: &importsv1alpha1.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
						Begin: "123",
						End:   "1234",
					},
					HVPAConfigurationHPA: &importsv1alpha1.HVPAConfigurationHPA{
						MinReplicas:                    pointer.Int32(1),
						MaxReplicas:                    pointer.Int32(4),
						TargetAverageUtilizationCpu:    pointer.Int32(80),
						TargetAverageUtilizationMemory: pointer.Int32(10),
					},
					HVPAConfigurationVPA: &importsv1alpha1.HVPAConfigurationVPA{
						ScaleUpMode:   pointer.String("Auto"),
						ScaleDownMode: pointer.String("Off"),
						ScaleUpStabilization: &hvpav1alpha1.ScaleType{
							UpdatePolicy: hvpav1alpha1.UpdatePolicy{
								UpdateMode: pointer.String("Auto"),
							},
							MinChange: hvpav1alpha1.ScaleParams{
								CPU: hvpav1alpha1.ChangeParams{
									Value:      pointer.String("80"),
									Percentage: pointer.Int32(10),
								},
								Memory: hvpav1alpha1.ChangeParams{
									Value:      pointer.String("80"),
									Percentage: pointer.Int32(10),
								},
								Replicas: hvpav1alpha1.ChangeParams{
									Value:      pointer.String("80"),
									Percentage: pointer.Int32(10),
								},
							},
							StabilizationDuration: pointer.String("10s"),
						},
						ScaleDownStabilization: &hvpav1alpha1.ScaleType{
							UpdatePolicy: hvpav1alpha1.UpdatePolicy{
								UpdateMode: pointer.String("Off"),
							},
							MinChange:             hvpav1alpha1.ScaleParams{},
							StabilizationDuration: pointer.String("10s"),
						},
						LimitsRequestsGapScaleParams: &hvpav1alpha1.ScaleParams{
							CPU: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("80"),
								Percentage: pointer.Int32(10),
							},
							Memory: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("80"),
								Percentage: pointer.Int32(10),
							},
							Replicas: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("80"),
								Percentage: pointer.Int32(10),
							},
						},
					},
				},
			},
			ComponentConfiguration: importsv1alpha1.APIServerComponentConfiguration{
				ClusterIdentity: &clusterIdentity,
				Encryption: &apiserverconfigv1.EncryptionConfiguration{
					Resources: []apiserverconfigv1.ResourceConfiguration{
						{
							Resources: []string{"plants", "secrets"},
						},
					},
				},
				Etcd: importsv1alpha1.APIServerEtcdConfiguration{
					Url:        etcdUrl,
					CABundle:   &etcdCACrt,
					ClientCert: &etcdClientCert,
					ClientKey:  &etcdClientKey,
				},
				CA: &importsv1alpha1.CA{
					Crt: &defaultCACrt,
					Key: &defaultCAKey,
				},
				TLS: &importsv1alpha1.TLSServer{
					Crt: &defaultTLSCrt,
					Key: &defaultTLSKey,
				},
				FeatureGates: map[string]bool{
					"feature": true,
				},
				Admission: &importsv1alpha1.APIServerAdmissionConfiguration{
					EnableAdmissionPlugins: []string{
						"my-test-plugin",
					},
					DisableAdmissionPlugins: []string{
						"my-disabled-test-plugin",
					},
					Plugins: []apiserverv1.AdmissionPluginConfiguration{
						{
							Name: "my-test-plugin",
							Path: "a/b/c",
						},
					},
					ValidatingWebhook: &importsv1alpha1.APIServerAdmissionWebhookCredentials{
						Kubeconfig: &landscaperv1alpha1.Target{Spec: landscaperv1alpha1.TargetSpec{Configuration: landscaperv1alpha1.AnyJSON{
							RawMessage: []byte(getVolumeProjectionKubeconfig("validating")),
						}}},
						TokenProjection: &importsv1alpha1.APIServerAdmissionWebhookCredentialsTokenProjection{
							Enabled:           true,
							Audience:          pointer.String("gardener"),
							ExpirationSeconds: pointer.Int32(30),
						},
					},
					MutatingWebhook: &importsv1alpha1.APIServerAdmissionWebhookCredentials{
						Kubeconfig: &landscaperv1alpha1.Target{Spec: landscaperv1alpha1.TargetSpec{Configuration: landscaperv1alpha1.AnyJSON{
							RawMessage: []byte(getVolumeProjectionKubeconfig("mutating")),
						}}},
						TokenProjection: &importsv1alpha1.APIServerAdmissionWebhookCredentialsTokenProjection{
							Enabled:           true,
							Audience:          pointer.String("gardener"),
							ExpirationSeconds: pointer.Int32(30),
						},
					},
				},
				GoAwayChance:                 pointer.Float32(10),
				Http2MaxStreamsPerConnection: pointer.Int32(5),
				ShutdownDelayDuration:        &metav1.Duration{Duration: 20 * time.Second},
				Requests: &importsv1alpha1.APIServerRequests{
					MaxNonMutatingInflight: pointer.Int(5),
					MaxMutatingInflight:    pointer.Int(10),
					MinTimeout:             &metav1.Duration{Duration: 5 * time.Second},
					Timeout:                &metav1.Duration{Duration: 30 * time.Second},
				},
				WatchCacheSize: &importsv1alpha1.APIServerWatchCacheConfiguration{
					DefaultSize: pointer.Int32(100),
					Resources: []importsv1alpha1.WatchCacheSizeResource{
						{
							ApiGroup: "core.gardener",
							Resource: "Shoot",
							Size:     15,
						},
					},
				},
				Audit: &importsv1alpha1.APIServerAuditConfiguration{
					Policy: &auditv1.Policy{
						Rules: []auditv1.PolicyRule{
							{
								Level: "info",
							},
						},
					},
					Log: &importsv1alpha1.APIServerAuditLogBackend{
						APIServerAuditCommonBackendConfiguration: importsv1alpha1.APIServerAuditCommonBackendConfiguration{
							BatchBufferSize: pointer.Int32(1),
							BatchMaxSize:    pointer.Int32(1),
							BatchMaxWait: &metav1.Duration{
								Duration: 1 * time.Second,
							},
							BatchThrottleBurst:   pointer.Int32(1),
							BatchThrottleEnable:  pointer.Bool(true),
							BatchThrottleQPS:     pointer.Float32(3.0),
							Mode:                 pointer.String("batch"),
							TruncateEnabled:      pointer.Bool(true),
							TruncateMaxBatchSize: pointer.Int32(1),
							TruncateMaxEventSize: pointer.Int32(1),
							Version:              pointer.String("some valid  version"),
						},
						Format:    pointer.String("json"),
						MaxBackup: pointer.Int32(1),
						MaxSize:   pointer.Int32(1),
						Path:      pointer.String("path"),
					},
					Webhook: &importsv1alpha1.APIServerAuditWebhookBackend{
						Kubeconfig: landscaperv1alpha1.Target{
							Spec: landscaperv1alpha1.TargetSpec{
								Configuration: landscaperv1alpha1.AnyJSON{},
							},
						},
						InitialBackoff: &metav1.Duration{
							Duration: 1 * time.Second,
						},
					},
				},
			},
		}

		admissionControllerConfigurationOrig = importsv1alpha1.GardenerAdmissionController{
			Enabled:         true,
			SeedRestriction: &importsv1alpha1.SeedRestriction{Enabled: true},
			DeploymentConfiguration: &importsv1alpha1.CommonDeploymentConfiguration{
				ReplicaCount:       pointer.Int32(1),
				ServiceAccountName: pointer.String("sx"),
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"cpu": resource.MustParse("2"),
					},
					Requests: corev1.ResourceList{
						"memory": resource.MustParse("3Gi"),
					},
				},
				PodLabels:      map[string]string{"foo": "bar"},
				PodAnnotations: map[string]string{"foo": "annotation"},
				VPA:            pointer.Bool(true),
			},
			ComponentConfiguration: &importsv1alpha1.AdmissionControllerComponentConfiguration{
				CA: &importsv1alpha1.CA{
					Crt: &defaultCACrt,
					Key: &defaultCAKey,
				},
				TLS: &importsv1alpha1.TLSServer{
					Crt: &defaultTLSCrt,
					Key: &defaultTLSKey,
				},
			},
		}

		hostPathType                       = corev1.HostPathFileOrCreate
		controllerManagerConfigurationOrig = importsv1alpha1.GardenerControllerManager{
			DeploymentConfiguration: &importsv1alpha1.ControllerManagerDeploymentConfiguration{
				CommonDeploymentConfiguration: &importsv1alpha1.CommonDeploymentConfiguration{
					ReplicaCount:       pointer.Int32(1),
					ServiceAccountName: pointer.String("sx"),
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"cpu": resource.MustParse("2"),
						},
						Requests: corev1.ResourceList{
							"memory": resource.MustParse("3Gi"),
						},
					},
					PodLabels:      map[string]string{"foo": "bar"},
					PodAnnotations: map[string]string{"foo": "annotation"},
					VPA:            pointer.Bool(true),
				},
				AdditionalVolumes: []corev1.Volume{
					{
						Name: "voluminous",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/usr/local/path",
								Type: &hostPathType,
							},
						},
					},
				},
				AdditionalVolumeMounts: []corev1.VolumeMount{
					{
						Name:      "voluminous",
						ReadOnly:  true,
						MountPath: "/usr/local/path",
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  "KUBECONFIG",
						Value: "/usr/local/here.config",
					},
				},
			},
			ComponentConfiguration: &importsv1alpha1.ControllerManagerComponentConfiguration{
				TLS: &importsv1alpha1.TLSServer{
					Crt: &defaultTLSCrt,
					Key: &defaultTLSKey,
				},
			},
		}

		schedulerConfiguration = importsv1alpha1.GardenerScheduler{
			DeploymentConfiguration: &importsv1alpha1.CommonDeploymentConfiguration{
				ReplicaCount:       pointer.Int32(1),
				ServiceAccountName: pointer.String("sx"),
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"cpu": resource.MustParse("2"),
					},
					Requests: corev1.ResourceList{
						"memory": resource.MustParse("3Gi"),
					},
				},
				PodLabels:      map[string]string{"foo": "bar"},
				PodAnnotations: map[string]string{"foo": "annotation"},
				VPA:            pointer.Bool(true),
			},
		}

		defaultImage = Image{
			Repository: "test",
			Tag:        "repo",
		}

		secretNameEtcdTLS              = "etcd"
		secretNameAPIServerTLS         = "apiserverTLS"
		secretNameAdmissionTLS         = "admissionTLS"
		secretNameControllerManagerTLS = "controllerTLS"
	)

	BeforeEach(func() {
		// make sure values are reset
		controllerManagerConfiguration := controllerManagerConfigurationOrig
		admissionControllerConfiguration := admissionControllerConfigurationOrig
		apiserverConfiguration := apiserverConfigurationOrig

		testTargetConfigBytes, err = json.Marshal(testTargetConfig)
		Expect(err).ToNot(HaveOccurred())
		apiserverConfiguration.ComponentConfiguration.Audit.Webhook.Kubeconfig.Spec.Configuration.RawMessage = testTargetConfigBytes

		getValuesHelper = func(virtualGardenEnabled, useSecretReferences bool) RuntimeChartValuesHelper {
			if useSecretReferences {
				apiserverConfiguration.ComponentConfiguration.Etcd.SecretRef = &corev1.SecretReference{
					Name:      secretNameEtcdTLS,
					Namespace: "garden",
				}

				apiserverConfiguration.ComponentConfiguration.TLS.SecretRef = &corev1.SecretReference{
					Name:      secretNameAPIServerTLS,
					Namespace: "garden",
				}

				admissionControllerConfiguration.ComponentConfiguration.TLS.SecretRef = &corev1.SecretReference{
					Name:      secretNameAdmissionTLS,
					Namespace: "garden",
				}

				controllerManagerConfiguration.ComponentConfiguration.TLS.SecretRef = &corev1.SecretReference{
					Name:      secretNameControllerManagerTLS,
					Namespace: "garden",
				}
			}

			return NewRuntimeChartValuesHelper(
				clusterIdentity,
				virtualGardenEnabled,
				&rbac,
				&clusterIP,
				&kubeconfigAPIServer,
				&kubeconfigControllerManager,
				&kubeconfigScheduler,
				&kubeconfigAdmissionController,
				admissionComponentConfig,
				&gcmComponentConfig,
				&schedulerComponentConfig,
				apiserverConfiguration,
				controllerManagerConfiguration,
				admissionControllerConfiguration,
				schedulerConfiguration,
				defaultImage,
				defaultImage,
				defaultImage,
				defaultImage,
			)
		}

		expectedValues = func(virtualGardenEnabled bool, useSecretReferences bool) map[string]interface{} {
			result := map[string]interface{}{
				"global": map[string]interface{}{
					"deployment": map[string]interface{}{
						"virtualGarden": map[string]interface{}{
							"clusterIP": "10.0.1.1",
							"enabled":   virtualGardenEnabled,
						},
					},
					"apiserver": map[string]interface{}{
						"kubeconfig": pointer.String("kubecfg-apiserver"),
						"image": map[string]interface{}{
							"repository": "test",
							"tag":        "repo",
						},

						// deployment values
						"podAnnotations": map[string]interface{}{
							"foo": "annotation",
						},
						"vpa": true,
						"resources": map[string]interface{}{
							"limits": map[string]interface{}{
								"cpu": "2",
							},
							"requests": map[string]interface{}{
								"memory": "3Gi",
							},
						},
						"replicaCount":    float64(1),
						"podLabels":       map[string]interface{}{"foo": "bar"},
						"minReadySeconds": float64(1),
						"readinessProbe": map[string]interface{}{
							"initialDelaySeconds": float64(5),
							"timeoutSeconds":      float64(10),
							"periodSeconds":       float64(15),
						},
						"livenessProbe": map[string]interface{}{
							"initialDelaySeconds": float64(5),
							"timeoutSeconds":      float64(10),
							"periodSeconds":       float64(15),
						},
						"serviceAccountName": "sx",
						"hvpa": map[string]interface{}{
							"enabled": true,
							"maintenanceWindow": map[string]interface{}{
								"begin": "123",
								"end":   "1234",
							},
							"maxReplicas":                    int32(4),
							"minReplicas":                    int32(1),
							"targetAverageUtilizationCpu":    int32(80),
							"targetAverageUtilizationMemory": int32(10),
							"vpaScaleUpMode":                 "Auto",
							"vpaScaleDownMode":               pointer.String("Off"),
							"vpaScaleUpStabilization": map[string]interface{}{
								"stabilizationDuration": "10s",
								"updatePolicy": map[string]interface{}{
									"updateMode": "Auto",
								},
								"minChange": map[string]interface{}{
									"cpu": map[string]interface{}{
										"value":      "80",
										"percentage": float64(10),
									},
									"memory": map[string]interface{}{
										"value":      "80",
										"percentage": float64(10),
									},
									"replicas": map[string]interface{}{
										"percentage": float64(10),
										"value":      "80",
									},
								},
							},
							"vpaScaleDownStabilization": map[string]interface{}{
								"stabilizationDuration": "10s",
								"updatePolicy": map[string]interface{}{
									"updateMode": "Off",
								},
								"minChange": map[string]interface{}{
									"cpu":      map[string]interface{}{},
									"memory":   map[string]interface{}{},
									"replicas": map[string]interface{}{},
								},
							},
							"limitsRequestsGapScaleParams": map[string]interface{}{
								"cpu": map[string]interface{}{
									"value":      "80",
									"percentage": float64(10),
								},
								"memory": map[string]interface{}{
									"value":      "80",
									"percentage": float64(10),
								},
								"replicas": map[string]interface{}{
									"value":      "80",
									"percentage": float64(10),
								},
							},
						},

						// component values
						"encryption": map[string]interface{}{
							"config": "apiVersion: apiserver.config.k8s.io/v1\nkind: EncryptionConfiguration\nresources:\n- providers: null\n  resources:\n  - plants\n  - secrets\n",
						},

						"goAwayChance":                 pointer.Float32(10),
						"shutdownDelayDuration":        "20s",
						"http2MaxStreamsPerConnection": pointer.Int32(5),
						"tls": map[string]interface{}{
							"crt": "tlsCert",
							"key": "tlsKey",
						},
						"clusterIdentity": "landscape-one",
						"audit": map[string]interface{}{
							"policy": "{\"kind\":\"Policy\",\"apiVersion\":\"audit.k8s.io/v1\",\"metadata\":{\"creationTimestamp\":null},\"rules\":[{\"level\":\"info\"}]}\n",
							"log": map[string]interface{}{
								"batchThrottleQPS":     float64(3),
								"maxSize":              float64(1),
								"batchThrottleBurst":   float64(1),
								"maxBackup":            float64(1),
								"truncateMaxBatchSize": "1",
								"mode":                 "batch",
								"truncateMaxEventSize": "1",
								"version":              "some valid  version",
								"format":               "json",
								"batchThrottleEnable":  true,
								"truncateEnabled":      true,
								"path":                 "path",
								"batchBufferSize":      "1",
								"batchMaxWait":         "1s",
								"batchMaxSize":         float64(1),
							},
							"webhook": map[string]interface{}{
								"config":         "apiVersion: landscaper.gardener.cloud/v1alpha1\nkind: Target\nspec:\ntype: landscaper.gardener.cloud/kubernetes-cluster\nconfig:\n  kubeconfig: |\n\t---\n\tapiVersion: v1\n\tclusters:\n\t  - cluster:\n\t\t  certificate-authority-data: fff\n\t\t  server: https://m\n\t\tname: sdf\n\tcontexts:\n\t  - context:\n\t\t  cluster: v\n\t\t  user: d\n\t\tname: b\n\tcurrent-context: shoot--garden-ls\n\tkind: Config\n\tpreferences: {}\n\tusers:\n\t  - name: abc\n\t\tuser:\n\t\t  token: abc",
								"initialBackoff": "1s",
							},
						},
						"etcd": map[string]interface{}{
							"servers":  "etcd.svc.local:2273",
							"caBundle": "caCertEtcd",
							"tls": map[string]interface{}{
								"crt": pointer.String("etcdClientCert"),
								"key": pointer.String("etcdClientKey"),
							},
						},
						"featureGates": map[string]bool{"feature": true},

						"disableAdmissionPlugins": []string{
							"my-disabled-test-plugin",
						},
						"enableAdmissionPlugins": []string{"my-test-plugin"},
						"caBundle":               "caCert",
						"requests": map[string]interface{}{
							"maxMutatingInflight":    float64(10),
							"minTimeout":             "5s",
							"timeout":                "30s",
							"maxNonMutatingInflight": float64(5),
						},
						"plugins": map[string]interface{}{
							"my-test-plugin": map[string]interface{}{
								"name":          "my-test-plugin",
								"path":          "a/b/c",
								"configuration": nil,
							},
						},
						"watchCacheSizes": map[string]interface{}{
							"defaultSize": float64(100),
							"resources": []interface{}{
								map[string]interface{}{
									"apiGroup": "core.gardener",
									"resource": "Shoot",
									"size":     float64(15),
								},
							},
						},
					},
					"admission": map[string]interface{}{
						"seedRestriction": map[string]interface{}{
							"enabled": true,
						},
						"kubeconfig": pointer.String("kubecfg-admission"),
						"image": map[string]interface{}{
							"repository": "test",
							"tag":        "repo",
						},

						// deployment values
						"podAnnotations": map[string]interface{}{
							"foo": "annotation",
						},
						"vpa": true,
						"resources": map[string]interface{}{
							"limits": map[string]interface{}{
								"cpu": "2",
							},
							"requests": map[string]interface{}{
								"memory": "3Gi",
							},
						},
						"replicaCount":       float64(1),
						"podLabels":          map[string]interface{}{"foo": "bar"},
						"serviceAccountName": "sx",

						// component values
						"config": map[string]interface{}{
							"server": map[string]interface{}{
								"https": map[string]interface{}{
									"tls": map[string]interface{}{
										"crt":      "tlsCert",
										"key":      "tlsKey",
										"caBundle": "caCert",
									},
								},
								"resourceAdmissionConfiguration": map[string]interface{}{
									"limits": []interface{}{
										map[string]interface{}{"size": "1"},
									},
								},
							},
							"gardenClientConnection": map[string]interface{}{},
						},
					},
					"controller": map[string]interface{}{
						"kubeconfig": pointer.String("kubecfg-gcm"),
						"image": map[string]interface{}{
							"repository": "test",
							"tag":        "repo",
						},

						// deployment values
						"podAnnotations": map[string]interface{}{
							"foo": "annotation",
						},
						"vpa": true,
						"resources": map[string]interface{}{
							"limits": map[string]interface{}{
								"cpu": "2",
							},
							"requests": map[string]interface{}{
								"memory": "3Gi",
							},
						},
						"replicaCount":       float64(1),
						"podLabels":          map[string]interface{}{"foo": "bar"},
						"serviceAccountName": "sx",
						"additionalVolumeMounts": []interface{}{
							map[string]interface{}{
								"name":      "voluminous",
								"readOnly":  true,
								"mountPath": "/usr/local/path",
							},
						},
						"additionalVolumes": []interface{}{
							map[string]interface{}{
								"name": "voluminous",
								"hostPath": map[string]interface{}{
									"path": "/usr/local/path",
									"type": "FileOrCreate",
								},
							},
						},
						"env": []interface{}{
							map[string]interface{}{
								"name":  "KUBECONFIG",
								"value": "/usr/local/here.config",
							},
						},
						"config": map[string]interface{}{
							"controllers": map[string]interface{}{
								"bastion": map[string]interface{}{
									"concurrentSyncs": float64(10),
								},
								"shootMaintenance": map[string]interface{}{},
								"shootQuota": map[string]interface{}{
									"syncPeriod": "0s",
								},
								"shootHibernation": map[string]interface{}{},
							},
							"gardenClientConnection": map[string]interface{}{},
							"server": map[string]interface{}{
								"http": map[string]interface{}{},
								"https": map[string]interface{}{
									"tls": map[string]interface{}{
										"crt": "tlsCert",
										"key": "tlsKey",
									},
								},
							},
						},
					},
					"scheduler": map[string]interface{}{
						"kubeconfig": pointer.String("kubecfg-scheduler"),
						"image": map[string]interface{}{
							"repository": "test",
							"tag":        "repo",
						},

						// deployment values
						"podAnnotations": map[string]interface{}{
							"foo": "annotation",
						},
						"vpa": true,
						"resources": map[string]interface{}{
							"limits": map[string]interface{}{
								"cpu": "2",
							},
							"requests": map[string]interface{}{
								"memory": "3Gi",
							},
						},
						"replicaCount":       float64(1),
						"podLabels":          map[string]interface{}{"foo": "bar"},
						"serviceAccountName": "sx",

						// config
						"config": map[string]interface{}{
							"clientConnection": map[string]interface{}{},
							"server":           map[string]interface{}{},
							"schedulers": map[string]interface{}{
								"shoot": map[string]interface{}{
									"concurrentSyncs": float64(20),
								},
							},
							"featureGates": map[string]interface{}{
								"gago-feature": true,
							},
						},
					},
					"rbac": map[string]interface{}{
						"seedAuthorizer": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}

			if useSecretReferences {
				result, _ = utils.SetToValuesMap(result, secretNameEtcdTLS, "global", "apiserver", "etcd", "tlsSecretName")
				result, _ = utils.DeleteFromValuesMap(result, "global", "apiserver", "etcd", "tls")

				result, _ = utils.SetToValuesMap(result, secretNameAPIServerTLS, "global", "apiserver", "tlsSecretName")
				result, _ = utils.DeleteFromValuesMap(result, "global", "apiserver", "tls")

				result, _ = utils.SetToValuesMap(result, secretNameAdmissionTLS, "global", "admission", "config", "server", "https", "tlsSecretName")
				result, _ = utils.DeleteFromValuesMap(result, "global", "admission", "config", "server", "https", "tls", "crt")
				result, _ = utils.DeleteFromValuesMap(result, "global", "admission", "config", "server", "https", "tls", "key")

				result, _ = utils.SetToValuesMap(result, secretNameControllerManagerTLS, "global", "controller", "config", "server", "https", "tlsSecretName")
				result, _ = utils.DeleteFromValuesMap(result, "global", "controller", "config", "server", "https", "tls", "crt")
				result, _ = utils.DeleteFromValuesMap(result, "global", "controller", "config", "server", "https", "tls", "key")
			}

			return result
		}
	})

	Describe("#GetRuntimeChartValues", func() {
		It("should compute the correct runtime chart values - virtual garden", func() {
			result, err := getValuesHelper(true, false).GetRuntimeChartValues()
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expectedValues(true, false)))
		})
		It("should compute the correct runtime chart values - no virtual garden", func() {
			result, err := getValuesHelper(false, false).GetRuntimeChartValues()
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expectedValues(false, false)))
		})

		It("should compute the correct runtime chart values - use secret references for certificates", func() {
			result, err := getValuesHelper(true, true).GetRuntimeChartValues()
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expectedValues(true, true)))
		})
	})

})

func getVolumeProjectionKubeconfig(name string) string {
	return fmt.Sprintf(`
---
apiVersion: v1
kind: Config
users:
- name: '*'
user:
  tokenFile: /var/run/secrets/admission-tokens/%s-webhook-token`, name)
}
