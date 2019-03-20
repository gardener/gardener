/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package resources

import (
	"fmt"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sync"
)

func init() {
	Register(corev1.SchemeBuilder)
	Register(extensions.SchemeBuilder)
	Register(apps.SchemeBuilder)
}

var lock sync.Mutex

///////////////////////////////////////////////////////////////////////////////
// Explcit version mappings for api groups to use for resources

var defaultVersions = map[string]string{}

func DeclareDefaultVersion(gv schema.GroupVersion) {
	lock.Lock()
	defer lock.Unlock()

	if old, ok := defaultVersions[gv.Group]; ok {
		panic(fmt.Sprintf("default version for %s already set to %s", gv, old))
	}
	defaultVersions[gv.Group] = gv.Version
}

func DefaultVersion(g string) string {
	lock.Lock()
	defer lock.Unlock()
	return defaultVersions[g]
}

///////////////////////////////////////////////////////////////////////////////
// registration of default schemes for info management

var scheme = runtime.NewScheme()

func Register(builders ...runtime.SchemeBuilder) {
	lock.Lock()
	defer lock.Unlock()
	for _, b := range builders {
		b.AddToScheme(scheme)
	}
}

func DefaultScheme() *runtime.Scheme {
	return scheme
}
