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
	"bytes"
	"fmt"
	"strconv"

	importsv1alpha1 "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1"
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/etcdencryption"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

// Image represents an OCI image in a registry
type Image struct {
	Repository string
	Tag        string
}

// RuntimeChartValuesHelper provides methods computing the values to be used when applying the control plane runtime chart
type RuntimeChartValuesHelper interface {
	// GetRuntimeChartValues computes the values to be used when applying the control plane runtime chart.
	GetRuntimeChartValues() (map[string]interface{}, error)
}

// runtimeValuesHelper is a concrete implementation of RuntimeChartValuesHelper
// Contains all values that are needed to render the control plane runtime chart
type runtimeValuesHelper struct {
	// Etcd contains the etcd configuration for the Gardener API Server
	Etcd importsv1alpha1.Etcd
	// ClusterIdentity is the identity of the Gardener Installation configured in the Gardener API Server configuration
	ClusterIdentity string
	// UseVirtualGarden defines if the application chart is installed into a virtual Garden cluster
	// This causes the service 'gardener-apiserver' in the runtime cluster to contain a clusterIP that is referenced by the
	// endpoint 'gardener-apiserver' in the virtual garden cluster.
	UseVirtualGarden bool // .Values.global.deployment.virtualGarden.enabled
	// VirtualGardenClusterIP is the ClusterIP of the Gardener API Server service in the runtime cluster corresponding to
	// the fixed endpoints resource with the same IP of the "gardener-apiserver" service in the virtual garden.
	VirtualGardenClusterIP *string
	// VirtualGardenKubeconfigGardenerAPIServer is the generated Kubeconfig for the Gardener API Server
	// Only set when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerAPIServer *string
	// VirtualGardenKubeconfigGardenerControllerManager is the generated Kubeconfig for the Gardener Controller Manager
	// Only set when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerControllerManager *string
	// VirtualGardenKubeconfigGardenerScheduler is the generated Kubeconfig for the Gardener Scheduler
	// Only set when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerScheduler *string
	// VirtualGardenKubeconfigGardenerAdmissionController is the generated Kubeconfig for the Gardener Admission Controller
	// Only set when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerAdmissionController *string
	// ComponentConfigAdmissionController is the component config of the Gardener Admission Controller
	// Needed for the runtime chart to deploy the config map containing the component configuration of the Gardener Admission Controller
	ComponentConfigAdmissionController *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration
	// ComponentConfigControllerManager is the component config of the Gardener Controller Manager
	// Needed for the runtime chart to deploy the config map containing the component configuration of the Gardener Controller Manager
	ComponentConfigControllerManager *controllermanagerconfigv1alpha1.ControllerManagerConfiguration
	// ComponentConfigScheduler is the component config of the Gardener Scheduler
	// Needed for the runtime chart to deploy the config map containing the component configuration of the Gardener Scheduler
	ComponentConfigScheduler *schedulerconfigv1alpha1.SchedulerConfiguration
	// ApiServerConfiguration is the import configuration for the Gardener API Server
	ApiServerConfiguration importsv1alpha1.GardenerAPIServer
	// AdmissionControllerConfiguration is the import configuration for the Gardener Admission Controller
	AdmissionControllerConfiguration importsv1alpha1.GardenerAdmissionController
	// ControllerManagerConfiguration is the import configuration for the Gardener Controller Manager
	ControllerManagerConfiguration importsv1alpha1.GardenerControllerManager
	// SchedulerConfiguration is the import configuration for the Gardener Scheduler
	SchedulerConfiguration importsv1alpha1.GardenerScheduler
	// Rbac configures common RBAC configuration
	Rbac *importsv1alpha1.Rbac
	// APIServerImage contains the image references for the Gardener API Server
	APIServerImage Image
	// ControllerManagerImage contains the image references for the Gardener Controller Manager
	ControllerManagerImage Image
	// SchedulerImage contains the image references for the Gardener Scheduler
	SchedulerImage Image
	// AdmissionControllerImage contains the image references for the Gardener Admission Controller
	AdmissionControllerImage Image
}

var encoder runtime.Encoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(auditv1.AddToScheme(scheme))
	codec := serializer.NewCodecFactory(scheme)
	si, ok := runtime.SerializerInfoForMediaType(codec.SupportedMediaTypes(), runtime.ContentTypeJSON)
	if !ok {
		panic(fmt.Sprintf("could not find encoder for media type %q to serialize the audit policy", runtime.ContentTypeJSON))
	}

	encoder = codec.EncoderForVersion(si.Serializer, auditv1.SchemeGroupVersion)
}

// NewRuntimeChartValuesHelper creates a new RuntimeChartValuesHelper.
func NewRuntimeChartValuesHelper(
	etcd importsv1alpha1.Etcd,
	clusterIdentity string,
	useVirtualGarden bool,
	rbac *importsv1alpha1.Rbac,
	virtualGardenClusterIP *string,
	virtualGardenKubeconfigGardenerAPIServer *string,
	virtualGardenKubeconfigGardenerControllerManager *string,
	virtualGardenKubeconfigGardenerScheduler *string,
	virtualGardenKubeconfigGardenerAdmissionController *string,
	admissionControllerConfig *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration,
	controllerManagerConfig *controllermanagerconfigv1alpha1.ControllerManagerConfiguration,
	schedulerConfig *schedulerconfigv1alpha1.SchedulerConfiguration,
	apiServerConfiguration importsv1alpha1.GardenerAPIServer,
	controllerManagerConfiguration importsv1alpha1.GardenerControllerManager,
	admissionControllerConfiguration importsv1alpha1.GardenerAdmissionController,
	schedulerConfiguration importsv1alpha1.GardenerScheduler,
	apiServerImage, controllerManagerImage, schedulerImage, admissionControllerImage Image) RuntimeChartValuesHelper {
	return &runtimeValuesHelper{
		Etcd: etcd,
		ClusterIdentity:                          clusterIdentity,
		UseVirtualGarden:                         useVirtualGarden,
		VirtualGardenClusterIP:                   virtualGardenClusterIP,
		Rbac:                                     rbac,
		VirtualGardenKubeconfigGardenerAPIServer: virtualGardenKubeconfigGardenerAPIServer,
		VirtualGardenKubeconfigGardenerControllerManager:   virtualGardenKubeconfigGardenerControllerManager,
		VirtualGardenKubeconfigGardenerScheduler:           virtualGardenKubeconfigGardenerScheduler,
		VirtualGardenKubeconfigGardenerAdmissionController: virtualGardenKubeconfigGardenerAdmissionController,
		ComponentConfigAdmissionController:                 admissionControllerConfig,
		ComponentConfigControllerManager:                   controllerManagerConfig,
		ComponentConfigScheduler:                           schedulerConfig,
		ApiServerConfiguration:                             apiServerConfiguration,
		AdmissionControllerConfiguration:                   admissionControllerConfiguration,
		ControllerManagerConfiguration:                     controllerManagerConfiguration,
		SchedulerConfiguration:                             schedulerConfiguration,
		APIServerImage:                                     apiServerImage,
		ControllerManagerImage:                             controllerManagerImage,
		SchedulerImage:                                     schedulerImage,
		AdmissionControllerImage:                           admissionControllerImage,
	}
}

func (v runtimeValuesHelper) GetRuntimeChartValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)

	values, err = utils.SetToValuesMap(values, v.UseVirtualGarden, "deployment", "virtualGarden", "enabled")
	if err != nil {
		return nil, err
	}

	if v.VirtualGardenClusterIP != nil {
		values, err = utils.SetToValuesMap(values, *v.VirtualGardenClusterIP, "deployment", "virtualGarden", "clusterIP")
		if err != nil {
			return nil, err
		}
	}

	values, err = utils.SetToValuesMap(values, v.UseVirtualGarden, "deployment", "virtualGarden", "enabled")
	if err != nil {
		return nil, err
	}

	apiserverDeploymentValues, err := v.getAPIServerDeploymentValues()
	if err != nil {
		return nil, err
	}

	apiserverComponentValues, err := v.getAPIServerComponentValues()
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, utils.MergeMaps(apiserverDeploymentValues, apiserverComponentValues), "apiserver")
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, v.APIServerImage.Repository, "apiserver", "image", "repository")
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, v.APIServerImage.Tag, "apiserver", "image", "tag")
	if err != nil {
		return nil, err
	}

	// Gardener Admission Controller Values

	values, err = utils.SetToValuesMap(values, v.AdmissionControllerConfiguration.Enabled, "admission", "enabled")
	if err != nil {
		return nil, err
	}

	if v.AdmissionControllerConfiguration.Enabled {
		admissionControllerDeploymentValues, err := v.getAdmissionControllerDeploymentValues()
		if err != nil {
			return nil, err
		}

		values, err = utils.SetToValuesMap(values, admissionControllerDeploymentValues, "admission")
		if err != nil {
			return nil, err
		}

		admissionControllerComponentValues, err := v.getAdmissionControllerComponentValues()
		if err != nil {
			return nil, err
		}

		values, err = utils.SetToValuesMap(values, admissionControllerComponentValues, "admission", "config")
		if err != nil {
			return nil, err
		}

		if v.AdmissionControllerConfiguration.SeedRestriction != nil {
			values, err = utils.SetToValuesMap(values, v.AdmissionControllerConfiguration.SeedRestriction.Enabled, "admission", "seedRestriction", "enabled")
			if err != nil {
				return nil, err
			}
		}

		if v.VirtualGardenKubeconfigGardenerAdmissionController != nil {
			values, err = utils.SetToValuesMap(values, v.VirtualGardenKubeconfigGardenerAdmissionController, "admission", "kubeconfig")
			if err != nil {
				return nil, err
			}
		}

		values, err = utils.SetToValuesMap(values, v.AdmissionControllerImage.Repository, "admission", "image", "repository")
		if err != nil {
			return nil, err
		}

		values, err = utils.SetToValuesMap(values, v.AdmissionControllerImage.Tag, "admission", "image", "tag")
		if err != nil {
			return nil, err
		}
	}

	// Gardener Controller Manager Values

	controllerManagerDeploymentValues, err := v.getControllerManagerDeploymentValues()
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, controllerManagerDeploymentValues, "controller")
	if err != nil {
		return nil, err
	}

	controllerManagerComponentValues, err := v.getControllerManagerComponentValues()
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, controllerManagerComponentValues, "controller", "config")
	if err != nil {
		return nil, err
	}

	if v.VirtualGardenKubeconfigGardenerControllerManager != nil {
		values, err = utils.SetToValuesMap(values, v.VirtualGardenKubeconfigGardenerControllerManager, "controller", "kubeconfig")
		if err != nil {
			return nil, err
		}
	}

	values, err = utils.SetToValuesMap(values, v.ControllerManagerImage.Repository, "controller", "image", "repository")
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, v.ControllerManagerImage.Tag, "controller", "image", "tag")
	if err != nil {
		return nil, err
	}

	// Gardener Scheduler Values

	schedulerDeploymentValues, err := v.getSchedulerDeploymentValues()
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, schedulerDeploymentValues, "scheduler")
	if err != nil {
		return nil, err
	}

	schedulerComponentValues, err := v.getSchedulerComponentValues()
	if err != nil {
		return nil, err
	}

	if schedulerComponentValues != nil {
		values, err = utils.SetToValuesMap(values, schedulerComponentValues, "scheduler", "config")
		if err != nil {
			return nil, err
		}
	}

	if v.VirtualGardenKubeconfigGardenerScheduler != nil {
		values, err = utils.SetToValuesMap(values, v.VirtualGardenKubeconfigGardenerScheduler, "scheduler", "kubeconfig")
		if err != nil {
			return nil, err
		}
	}

	values, err = utils.SetToValuesMap(values, v.SchedulerImage.Repository, "scheduler", "image", "repository")
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, v.SchedulerImage.Tag, "scheduler", "image", "tag")
	if err != nil {
		return nil, err
	}

	if v.Rbac != nil && v.Rbac.SeedAuthorizer != nil && v.Rbac.SeedAuthorizer.Enabled != nil {
		values, err = utils.SetToValuesMap(values, *v.Rbac.SeedAuthorizer.Enabled, "rbac", "seedAuthorizer", "enabled")
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"global": values,
	}, nil
}

func (v runtimeValuesHelper) getAPIServerDeploymentValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)

	if v.ApiServerConfiguration.DeploymentConfiguration != nil {
		// Convert deployment object to values
		values, err = utils.ToValuesMap(v.ApiServerConfiguration.DeploymentConfiguration)
		if err != nil {
			return nil, err
		}

		// cannot directly marshal the hvpa values
		values, err = utils.DeleteFromValuesMap(values, "hvpa")
		if err != nil {
			return nil, err
		}

		values, err = v.setHVPAValues(values, err)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func (v runtimeValuesHelper) setHVPAValues(values map[string]interface{}, err error) (map[string]interface{}, error) {
	if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa != nil && v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.Enabled != nil && *v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.Enabled {
		values, err = utils.SetToValuesMap(values, *v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.Enabled, "hvpa", "enabled")
		if err != nil {
			return nil, err
		}

		if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.MaintenanceTimeWindow != nil {
			mValues, err := utils.ToValuesMapWithOptions(*v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.MaintenanceTimeWindow, utils.Options{
				LowerCaseKeys: true,
			})
			if err != nil {
				return nil, err
			}

			values, err = utils.SetToValuesMap(values, mValues, "hvpa", "maintenanceWindow")
			if err != nil {
				return nil, err
			}
		}

		if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationHPA != nil {
			if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationHPA.MaxReplicas != nil {
				values, err = utils.SetToValuesMap(values, *v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationHPA.MaxReplicas, "hvpa", "maxReplicas")
				if err != nil {
					return nil, err
				}
			}

			if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationHPA.MinReplicas != nil {
				values, err = utils.SetToValuesMap(values, *v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationHPA.MinReplicas, "hvpa", "minReplicas")
				if err != nil {
					return nil, err
				}
			}

			if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationHPA.TargetAverageUtilizationCpu != nil {
				values, err = utils.SetToValuesMap(values, *v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationHPA.TargetAverageUtilizationCpu, "hvpa", "targetAverageUtilizationCpu")
				if err != nil {
					return nil, err
				}
			}

			if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationHPA.TargetAverageUtilizationMemory != nil {
				values, err = utils.SetToValuesMap(values, *v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationHPA.TargetAverageUtilizationMemory, "hvpa", "targetAverageUtilizationMemory")
				if err != nil {
					return nil, err
				}
			}
		}

		if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA != nil {
			if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.ScaleUpMode != nil {
				values, err = utils.SetToValuesMap(values, *v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.ScaleUpMode, "hvpa", "vpaScaleUpMode")
				if err != nil {
					return nil, err
				}
			}

			if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.ScaleDownMode != nil {
				values, err = utils.SetToValuesMap(values, v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.ScaleDownMode, "hvpa", "vpaScaleDownMode")
				if err != nil {
					return nil, err
				}
			}

			if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.ScaleUpStabilization != nil {
				scaleValues, err := utils.ToValuesMapWithOptions(*v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.ScaleUpStabilization, utils.Options{
					LowerCaseKeys: true,
				})
				if err != nil {
					return nil, err
				}

				values, err = utils.SetToValuesMap(values, scaleValues, "hvpa", "vpaScaleUpStabilization")
				if err != nil {
					return nil, err
				}
			}

			if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.ScaleDownStabilization != nil {
				scaleValues, err := utils.ToValuesMapWithOptions(*v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.ScaleDownStabilization, utils.Options{
					LowerCaseKeys: true,
				})
				if err != nil {
					return nil, err
				}

				values, err = utils.SetToValuesMap(values, scaleValues, "hvpa", "vpaScaleDownStabilization")
				if err != nil {
					return nil, err
				}
			}

			if v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.LimitsRequestsGapScaleParams != nil {
				limitValues, err := utils.ToValuesMapWithOptions(*v.ApiServerConfiguration.DeploymentConfiguration.Hvpa.HVPAConfigurationVPA.LimitsRequestsGapScaleParams, utils.Options{
					LowerCaseKeys: true,
				})
				if err != nil {
					return nil, err
				}

				values, err = utils.SetToValuesMap(values, limitValues, "hvpa", "limitsRequestsGapScaleParams")
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return values, nil
}

func (v runtimeValuesHelper) getAPIServerComponentValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)
	// return values, err
	if v.VirtualGardenKubeconfigGardenerAPIServer != nil {
		values, err = utils.SetToValuesMap(values, v.VirtualGardenKubeconfigGardenerAPIServer, "kubeconfig")
		if err != nil {
			return nil, err
		}
	}

	values, err = utils.SetToValuesMap(values, v.ClusterIdentity, "clusterIdentity")
	if err != nil {
		return nil, err
	}

	if v.ApiServerConfiguration.ComponentConfiguration.Encryption != nil {
		encryption, err := etcdencryption.Write(v.ApiServerConfiguration.ComponentConfiguration.Encryption)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal encryption configuration: %v", v.ApiServerConfiguration.ComponentConfiguration.Encryption)
		}

		values, err = utils.SetToValuesMap(values, string(encryption), "encryption", "config")
		if err != nil {
			return nil, err
		}
	}

	values, err = utils.SetToValuesMap(values, v.Etcd.EtcdUrl, "etcd", "servers")
	if err != nil {
		return nil, err
	}

	if v.Etcd.EtcdCABundle != nil {
		values, err = utils.SetToValuesMap(values, *v.Etcd.EtcdCABundle, "etcd", "caBundle")
		if err != nil {
			return nil, err
		}
	}

	// if a secret reference is specified, also use the secret reference name in the chart.
	// Otherwise, directly set the values
	if v.Etcd.EtcdSecretRef != nil {
		// the helm chart only has the option to set the name of the secret.
		// This is odd and should probably be changed in the chart.
		values, err = utils.SetToValuesMap(values, v.Etcd.EtcdSecretRef.Name, "etcd", "tlsSecretName")
		if err != nil {
			return nil, err
		}
	} else {
		values, err = utils.SetToValuesMap(values, v.Etcd.EtcdClientCert, "etcd", "tls", "crt")
		if err != nil {
			return nil, err
		}

		values, err = utils.SetToValuesMap(values, v.Etcd.EtcdClientKey, "etcd", "tls", "key")
		if err != nil {
			return nil, err
		}
	}

	values, err = utils.SetToValuesMap(values, *v.ApiServerConfiguration.ComponentConfiguration.CA.Crt, "caBundle")
	if err != nil {
		return nil, err
	}

	// if a secret reference is specified, also use the secret reference name in the chart.
	// Otherwise, directly set the values
	if v.ApiServerConfiguration.ComponentConfiguration.TLS.SecretRef != nil {
		// the helm chart only has the option to set the name of the secret.
		// This is odd and should probably be changed in the chart.
		values, err = utils.SetToValuesMap(values, v.ApiServerConfiguration.ComponentConfiguration.TLS.SecretRef.Name, "tlsSecretName")
		if err != nil {
			return nil, err
		}
	} else {
		if tlsValues, err := getTLSServerValues(v.ApiServerConfiguration.ComponentConfiguration.TLS); err == nil && len(tlsValues) > 0 {
			values, err = utils.SetToValuesMap(values, tlsValues, "tls")
			if err != nil {
				return nil, err
			}
		}
	}

	if len(v.ApiServerConfiguration.ComponentConfiguration.FeatureGates) > 0 {
		values, err = utils.SetToValuesMap(values, v.ApiServerConfiguration.ComponentConfiguration.FeatureGates, "featureGates")
		if err != nil {
			return nil, err
		}
	}

	if v.ApiServerConfiguration.ComponentConfiguration.Admission != nil {
		if len(v.ApiServerConfiguration.ComponentConfiguration.Admission.EnableAdmissionPlugins) > 0 {
			values, err = utils.SetToValuesMap(values, v.ApiServerConfiguration.ComponentConfiguration.Admission.EnableAdmissionPlugins, "enableAdmissionPlugins")
			if err != nil {
				return nil, err
			}
		}

		if len(v.ApiServerConfiguration.ComponentConfiguration.Admission.DisableAdmissionPlugins) > 0 {
			values, err = utils.SetToValuesMap(values, v.ApiServerConfiguration.ComponentConfiguration.Admission.DisableAdmissionPlugins, "disableAdmissionPlugins")
			if err != nil {
				return nil, err
			}
		}

		for _, plugin := range v.ApiServerConfiguration.ComponentConfiguration.Admission.Plugins {
			pluginValues, err := utils.ToValuesMap(plugin)
			if err != nil {
				return nil, err
			}

			values, err = utils.SetToValuesMap(values, pluginValues, "plugins", plugin.Name)
			if err != nil {
				return nil, err
			}
		}
	}

	if v.ApiServerConfiguration.ComponentConfiguration.GoAwayChance != nil {
		values, err = utils.SetToValuesMap(values, v.ApiServerConfiguration.ComponentConfiguration.GoAwayChance, "goAwayChance")
		if err != nil {
			return nil, err
		}
	}

	if v.ApiServerConfiguration.ComponentConfiguration.Http2MaxStreamsPerConnection != nil {
		values, err = utils.SetToValuesMap(values, v.ApiServerConfiguration.ComponentConfiguration.Http2MaxStreamsPerConnection, "http2MaxStreamsPerConnection")
		if err != nil {
			return nil, err
		}
	}

	if v.ApiServerConfiguration.ComponentConfiguration.ShutdownDelayDuration != nil {
		values, err = utils.SetToValuesMap(values, v.ApiServerConfiguration.ComponentConfiguration.ShutdownDelayDuration.Duration.String(), "shutdownDelayDuration")
		if err != nil {
			return nil, err
		}
	}

	if v.ApiServerConfiguration.ComponentConfiguration.Requests != nil {
		v, err := utils.ToValuesMapWithOptions(v.ApiServerConfiguration.ComponentConfiguration.Requests, utils.Options{RemoveZeroEntries: true, LowerCaseKeys: true})
		if err != nil {
			return nil, err
		}

		values, err = utils.SetToValuesMap(values, v, "requests")
		if err != nil {
			return nil, err
		}
	}

	if v.ApiServerConfiguration.ComponentConfiguration.WatchCacheSize != nil {
		v, err := utils.ToValuesMapWithOptions(v.ApiServerConfiguration.ComponentConfiguration.WatchCacheSize, utils.Options{RemoveZeroEntries: true, LowerCaseKeys: true})
		if err != nil {
			return nil, err
		}

		values, err = utils.SetToValuesMap(values, v, "watchCacheSizes")
		if err != nil {
			return nil, err
		}
	}

	// Audit *APIServerAuditConfiguration
	if v.ApiServerConfiguration.ComponentConfiguration.Audit != nil {
		val, err := utils.ToValuesMapWithOptions(v.ApiServerConfiguration.ComponentConfiguration.Audit, utils.Options{RemoveZeroEntries: true, LowerCaseKeys: true})
		if err != nil {
			return nil, err
		}

		values, err = utils.SetToValuesMap(values, val, "audit")
		if err != nil {
			return nil, err
		}

		if v.ApiServerConfiguration.ComponentConfiguration.Audit.Policy != nil {
			// the policy has to be set as a string
			utils.DeleteFromValuesMap(values, "audit", "policy")

			var b bytes.Buffer
			if err := encoder.Encode(v.ApiServerConfiguration.ComponentConfiguration.Audit.Policy, &b); err != nil {
				return nil, err
			}

			values, err = utils.SetToValuesMap(values, b.String(), "audit", "policy")
			if err != nil {
				return nil, err
			}
		}

		if v.ApiServerConfiguration.ComponentConfiguration.Audit.Webhook != nil {
			// the webhook kubeconfig has the key "config" in the helm chart, not "kubeconfig"
			// plus, we need the actual kubeconfig string, not the landscaper representation
			utils.DeleteFromValuesMap(values, "audit", "webhook", "kubeconfig")

			gardenClusterTargetConfig := &landscaperv1alpha1.KubernetesClusterTargetConfig{}
			if err := json.Unmarshal(v.ApiServerConfiguration.ComponentConfiguration.Audit.Webhook.Kubeconfig.Spec.Configuration.RawMessage, gardenClusterTargetConfig); err != nil {
				return nil, err
			}

			values, err = utils.SetToValuesMap(values, gardenClusterTargetConfig.Kubeconfig, "audit", "webhook", "config")
			if err != nil {
				return nil, err
			}

			// ensure is string, otherwise will be rendered in unsupported floating point notation
			if v.ApiServerConfiguration.ComponentConfiguration.Audit.Webhook.TruncateMaxBatchSize != nil {
				utils.SetToValuesMap(values, strconv.Itoa(int(*v.ApiServerConfiguration.ComponentConfiguration.Audit.Webhook.TruncateMaxBatchSize)), "audit", "webhook", "truncateMaxBatchSize")
			}

			if v.ApiServerConfiguration.ComponentConfiguration.Audit.Webhook.TruncateMaxEventSize != nil {
				utils.SetToValuesMap(values, strconv.Itoa(int(*v.ApiServerConfiguration.ComponentConfiguration.Audit.Webhook.TruncateMaxEventSize)), "audit", "webhook", "truncateMaxEventSize")
			}

			if v.ApiServerConfiguration.ComponentConfiguration.Audit.Webhook.BatchBufferSize != nil {
				utils.SetToValuesMap(values, strconv.Itoa(int(*v.ApiServerConfiguration.ComponentConfiguration.Audit.Webhook.BatchBufferSize)), "audit", "webhook", "batchBufferSize")
			}
		}

		if v.ApiServerConfiguration.ComponentConfiguration.Audit.Log != nil {
			// ensure is string, otherwise will be rendered in unsupported floating point notation
			if v.ApiServerConfiguration.ComponentConfiguration.Audit.Log.TruncateMaxBatchSize != nil {
				utils.SetToValuesMap(values, strconv.Itoa(int(*v.ApiServerConfiguration.ComponentConfiguration.Audit.Log.TruncateMaxBatchSize)), "audit", "log", "truncateMaxBatchSize")
			}

			if v.ApiServerConfiguration.ComponentConfiguration.Audit.Log.TruncateMaxEventSize != nil {
				utils.SetToValuesMap(values, strconv.Itoa(int(*v.ApiServerConfiguration.ComponentConfiguration.Audit.Log.TruncateMaxEventSize)), "audit", "log", "truncateMaxEventSize")
			}

			if v.ApiServerConfiguration.ComponentConfiguration.Audit.Log.BatchBufferSize != nil {
				utils.SetToValuesMap(values, strconv.Itoa(int(*v.ApiServerConfiguration.ComponentConfiguration.Audit.Log.BatchBufferSize)), "audit", "log", "batchBufferSize")
			}
		}
	}

	return values, nil
}

func (v runtimeValuesHelper) getAdmissionControllerComponentValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)

	if v.ComponentConfigAdmissionController != nil {
		values, err = utils.ToValuesMapWithOptions(*v.ComponentConfigAdmissionController, utils.Options{
			RemoveZeroEntries: true,
		})
		if err != nil {
			return nil, err
		}
	}

	// if a secret reference is specified, also use the secret reference name in the chart
	// Otherwise, directly set the values.
	if v.AdmissionControllerConfiguration.ComponentConfiguration.TLS.SecretRef != nil {
		// the helm chart only has the option to set the name of the secret.
		// This is odd and should probably be changed in the chart.
		values, err = utils.SetToValuesMap(values, v.AdmissionControllerConfiguration.ComponentConfiguration.TLS.SecretRef.Name, "server", "https", "tlsSecretName")
		if err != nil {
			return nil, err
		}
	} else {
		if tlsValues, err := getTLSServerValues(v.AdmissionControllerConfiguration.ComponentConfiguration.TLS); err == nil && len(tlsValues) > 0 {
			values, err = utils.SetToValuesMap(values, tlsValues, "server", "https", "tls")
			if err != nil {
				return nil, err
			}
		}
	}

	values, err = utils.SetToValuesMap(values, *v.AdmissionControllerConfiguration.ComponentConfiguration.CA.Crt, "server", "https", "tls", "caBundle")
	if err != nil {
		return nil, err
	}

	return values, nil
}

func (v runtimeValuesHelper) getControllerManagerComponentValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)

	// Remove empty map values. Otherwise, defaults from the helm chart are overwritten with empty values.
	// For example: if the `https.bindAddress is not explicitly configured, the default of 0.0.0.0 from the helm chart is not taken leading to a helm deployment failure.
	values, err = utils.ToValuesMapWithOptions(v.ComponentConfigControllerManager, utils.Options{
		RemoveZeroEntries: true,
	})
	if err != nil {
		return nil, err
	}

	// if a secret reference is specified, also use the secret reference name in the chart
	// Otherwise, directly set the values.
	if v.ControllerManagerConfiguration.ComponentConfiguration.TLS.SecretRef != nil {
		// the helm chart only has the option to set the name of the secret.
		// This is odd and should probably be changed in the chart.
		values, err = utils.SetToValuesMap(values, v.ControllerManagerConfiguration.ComponentConfiguration.TLS.SecretRef.Name, "server", "https", "tlsSecretName")
		if err != nil {
			return nil, err
		}
	} else {
		if tlsValues, err := getTLSServerValues(v.ControllerManagerConfiguration.ComponentConfiguration.TLS); err == nil && len(tlsValues) > 0 {
			values, err = utils.SetToValuesMap(values, tlsValues, "server", "https", "tls")
			if err != nil {
				return nil, err
			}
		}
	}

	return values, nil
}

func (v runtimeValuesHelper) getAdmissionControllerDeploymentValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)
	if v.AdmissionControllerConfiguration.DeploymentConfiguration != nil {
		values, err = utils.ToValuesMapWithOptions(v.AdmissionControllerConfiguration.DeploymentConfiguration, utils.Options{
			LowerCaseKeys: true,
		})
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func (v runtimeValuesHelper) getControllerManagerDeploymentValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)
	if v.ControllerManagerConfiguration.DeploymentConfiguration != nil {
		values, err = utils.ToValuesMap(v.ControllerManagerConfiguration.DeploymentConfiguration)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func (v runtimeValuesHelper) getSchedulerDeploymentValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)

	if v.SchedulerConfiguration.DeploymentConfiguration != nil {
		values, err = utils.ToValuesMap(v.SchedulerConfiguration.DeploymentConfiguration)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func (v runtimeValuesHelper) getSchedulerComponentValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)

	values, err = utils.ToValuesMapWithOptions(v.ComponentConfigScheduler, utils.Options{
		RemoveZeroEntries: true,
	})
	if err != nil {
		return nil, err
	}

	return values, nil
}

func getTLSServerValues(tls *importsv1alpha1.TLSServer) (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)

	if tls == nil {
		return values, nil
	}

	if tls.Crt != nil {
		values, err = utils.SetToValuesMap(values, *tls.Crt, "crt")
		if err != nil {
			return nil, err
		}
	}

	if tls.Key != nil {
		values, err = utils.SetToValuesMap(values, *tls.Key, "key")
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}
