// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
