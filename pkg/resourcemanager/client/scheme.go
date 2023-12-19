// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	apiextensionsinstall "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/install"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	apiregistrationinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

var (
	// SourceScheme is the scheme used in the source cluster.
	SourceScheme = runtime.NewScheme()
	// TargetScheme is the scheme used in the target cluster.
	TargetScheme = runtime.NewScheme()
	// CombinedScheme is the scheme used when the source cluster is equal to the target cluster.
	CombinedScheme = runtime.NewScheme()
)

func init() {
	var (
		sourceSchemeBuilder = runtime.NewSchemeBuilder(
			kubernetesscheme.AddToScheme,
			resourcesv1alpha1.AddToScheme,
			machinev1alpha1.AddToScheme,
			extensionsv1alpha1.AddToScheme,
			druidv1alpha1.AddToScheme,
		)
		targetSchemeBuilder = runtime.NewSchemeBuilder(
			kubernetesscheme.AddToScheme,
			hvpav1alpha1.AddToScheme,
			volumesnapshotv1.AddToScheme,
		)
	)

	utilruntime.Must(sourceSchemeBuilder.AddToScheme(SourceScheme))
	utilruntime.Must(sourceSchemeBuilder.AddToScheme(CombinedScheme))

	utilruntime.Must(targetSchemeBuilder.AddToScheme(TargetScheme))
	utilruntime.Must(targetSchemeBuilder.AddToScheme(CombinedScheme))

	apiextensionsinstall.Install(SourceScheme)
	apiextensionsinstall.Install(TargetScheme)
	apiregistrationinstall.Install(TargetScheme)
	apiextensionsinstall.Install(CombinedScheme)
	apiregistrationinstall.Install(CombinedScheme)
}
