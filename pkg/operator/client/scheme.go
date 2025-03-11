// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	monitoringv1beta1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apiextensionsinstall "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operationsinstall "github.com/gardener/gardener/pkg/apis/operations/install"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementinstall "github.com/gardener/gardener/pkg/apis/seedmanagement/install"
	settingsinstall "github.com/gardener/gardener/pkg/apis/settings/install"
)

var (
	// RuntimeScheme is the scheme used in the runtime cluster.
	RuntimeScheme = runtime.NewScheme()
	// VirtualScheme is the scheme used in the virtual cluster.
	VirtualScheme = runtime.NewScheme()

	// RuntimeSerializer is a YAML serializer using the Runtime scheme.
	RuntimeSerializer = json.NewSerializerWithOptions(json.DefaultMetaFactory, RuntimeScheme, RuntimeScheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	// RuntimeCodec is a codec factory using the Runtime scheme.
	RuntimeCodec = serializer.NewCodecFactory(RuntimeScheme)

	// VirtualSerializer is a YAML serializer using the Virtual scheme.
	VirtualSerializer = json.NewSerializerWithOptions(json.DefaultMetaFactory, VirtualScheme, VirtualScheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	// VirtualCodec is a codec factory using the Virtual scheme.
	VirtualCodec = serializer.NewCodecFactory(VirtualScheme)
)

var (
	runtimeSchemeBuilder = runtime.NewSchemeBuilder(
		kubernetesscheme.AddToScheme,
		operatorv1alpha1.AddToScheme,
		resourcesv1alpha1.AddToScheme,
		vpaautoscalingv1.AddToScheme,
		druidcorev1alpha1.AddToScheme,
		istionetworkingv1beta1.AddToScheme,
		istionetworkingv1alpha3.AddToScheme,
		extensionsv1alpha1.AddToScheme,
		monitoringv1.AddToScheme,
		monitoringv1beta1.AddToScheme,
		monitoringv1alpha1.AddToScheme,
		func(scheme *runtime.Scheme) error {
			apiextensionsinstall.Install(scheme)
			return nil
		},
	)
	virtualSchemeBuilder = runtime.NewSchemeBuilder(
		kubernetesscheme.AddToScheme,
		apiregistrationv1.AddToScheme,
		func(scheme *runtime.Scheme) error {
			apiextensionsinstall.Install(scheme)
			gardencoreinstall.Install(scheme)
			seedmanagementinstall.Install(scheme)
			settingsinstall.Install(scheme)
			operationsinstall.Install(scheme)
			return nil
		},
	)

	// AddRuntimeSchemeToScheme adds all object kinds used in the runtime cluster into the given scheme.
	AddRuntimeSchemeToScheme = runtimeSchemeBuilder.AddToScheme
	// AddVirtualSchemeToScheme adds all object kinds used in the Virtual cluster into the given scheme.
	AddVirtualSchemeToScheme = virtualSchemeBuilder.AddToScheme
)

func init() {
	utilruntime.Must(AddRuntimeSchemeToScheme(RuntimeScheme))
	utilruntime.Must(AddVirtualSchemeToScheme(VirtualScheme))
}
