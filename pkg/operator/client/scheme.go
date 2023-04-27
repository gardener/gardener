// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client

import (
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apiextensionsinstall "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

var (
	// RuntimeScheme is the scheme used in the runtime cluster.
	RuntimeScheme = runtime.NewScheme()
	// VirtualScheme is the scheme used in the virtual cluster.
	VirtualScheme = runtime.NewScheme()
)

func init() {
	var (
		runtimeSchemeBuilder = runtime.NewSchemeBuilder(
			kubernetesscheme.AddToScheme,
			operatorv1alpha1.AddToScheme,
			resourcesv1alpha1.AddToScheme,
			vpaautoscalingv1.AddToScheme,
			druidv1alpha1.AddToScheme,
			hvpav1alpha1.AddToScheme,
			istionetworkingv1beta1.AddToScheme,
			istionetworkingv1alpha3.AddToScheme,
		)
		virtualSchemeBuilder = runtime.NewSchemeBuilder(
			kubernetesscheme.AddToScheme,
		)
	)

	utilruntime.Must(runtimeSchemeBuilder.AddToScheme(RuntimeScheme))
	apiextensionsinstall.Install(RuntimeScheme)
	utilruntime.Must(virtualSchemeBuilder.AddToScheme(VirtualScheme))
	apiextensionsinstall.Install(VirtualScheme)
}
