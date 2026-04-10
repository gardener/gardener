// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// OSCDecoder can decode OperatingSystemConfig objects from raw bytes.
var OSCDecoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kubeletconfigv1beta1.AddToScheme(scheme))
	OSCDecoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}
