// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	monitoringv1beta1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// Decoder is a decoder for resources part of the `monitoring.coreos.com/v1{{beta,alpha}1} APIs.
var Decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1beta1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1alpha1.AddToScheme(scheme))
	Decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}
