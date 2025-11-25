// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	authenticationinstall "github.com/gardener/gardener/pkg/apis/authentication/install"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	operationsinstall "github.com/gardener/gardener/pkg/apis/operations/install"
	securityinstall "github.com/gardener/gardener/pkg/apis/security/install"
	seedmanagementinstall "github.com/gardener/gardener/pkg/apis/seedmanagement/install"
	settingsinstall "github.com/gardener/gardener/pkg/apis/settings/install"
)

var (
	// Scheme is a new API scheme.
	Scheme = runtime.NewScheme()
	// Codecs are used for serialization.
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	authenticationinstall.Install(Scheme)
	gardencoreinstall.Install(Scheme)
	securityinstall.Install(Scheme)
	seedmanagementinstall.Install(Scheme)
	settingsinstall.Install(Scheme)
	operationsinstall.Install(Scheme)

	utilruntime.Must(autoscalingv1.AddToScheme(Scheme))

	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})

	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}
