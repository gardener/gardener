// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Codec can encode or decode OperatingSystemConfig, KubeletConfiguration, or NodeAgentConfiguration objects to or from raw bytes.
var Codec runtime.Codec

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kubeletconfigv1beta1.AddToScheme(scheme))
	utilruntime.Must(nodeagentconfigv1alpha1.AddToScheme(scheme))

	ser := jsonserializer.NewSerializerWithOptions(jsonserializer.DefaultMetaFactory, scheme, scheme, jsonserializer.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	versions := schema.GroupVersions([]schema.GroupVersion{
		extensionsv1alpha1.SchemeGroupVersion,
		kubeletconfigv1beta1.SchemeGroupVersion,
		nodeagentconfigv1alpha1.SchemeGroupVersion,
	})
	Codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, versions, versions)
}
