// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sync"
)

var (
	configMapTypeMeta = metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"}

	configMapAPIResource = metav1.APIResource{
		Name:         "configmaps",
		SingularName: "configmap",
		Namespaced:   true,
		Kind:         "ConfigMap",
	}

	v1Group = metav1.APIGroup{
		Versions: []metav1.GroupVersionForDiscovery{{
			GroupVersion: "v1",
			Version:      "v1",
		}},
	}
)

type fakeDiscovery struct {
	*fakediscovery.FakeDiscovery
	lock          sync.Mutex
	groupListFn   func() *metav1.APIGroupList
	resourceMapFn func() map[string]*metav1.APIResourceList
}

func (c *fakeDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if rl, ok := c.resourceMapFn()[groupVersion]; ok {
		return rl, nil
	}
	return nil, errors.New("doesn't exist")
}

func (c *fakeDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	groupList := c.groupListFn()
	if groupList == nil {
		return nil, errors.New("doesn't exist")
	}
	return groupList, nil
}

func newTestApplier(c client.Client, discovery discovery.DiscoveryInterface) *kubernetes.Applier {
	tmp := kubernetes.NewControllerClient
	defer func() {
		kubernetes.NewControllerClient = tmp
	}()
	cachedDiscoveryClient := cached.NewMemCacheClient(discovery)
	kubernetes.NewControllerClient = func(config *rest.Config, options client.Options) (client.Client, error) {
		return c, nil
	}
	applier, err := kubernetes.NewApplierInternal(nil, cachedDiscoveryClient)
	Expect(err).NotTo(HaveOccurred())
	return applier
}

func mkManifest(objs ...runtime.Object) []byte {
	var out bytes.Buffer
	for _, obj := range objs {
		data, err := yaml.Marshal(obj)
		Expect(err).NotTo(HaveOccurred())
		out.Write(data)
		out.WriteString("---")
	}
	return out.Bytes()
}

var _ = Describe("Apply", func() {
	Context("Applier", func() {
		Context("#ApplyManifest", func() {
			It("should create non-existent objects", func() {
				c := fake.NewFakeClient()
				d := &fakeDiscovery{
					groupListFn: func() *metav1.APIGroupList {
						return &metav1.APIGroupList{
							Groups: []metav1.APIGroup{v1Group},
						}
					},
					resourceMapFn: func() map[string]*metav1.APIResourceList {
						return map[string]*metav1.APIResourceList{
							"v1": {
								GroupVersion: "v1",
								APIResources: []metav1.APIResource{configMapAPIResource},
							},
						}
					},
				}

				applier := newTestApplier(c, d)
				cm := corev1.ConfigMap{
					TypeMeta:   configMapTypeMeta,
					ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "c"},
				}
				manifest := mkManifest(&cm)

				Expect(applier.ApplyManifest(context.TODO(), manifest)).NotTo(HaveOccurred())
				var actual corev1.ConfigMap
				err := c.Get(context.TODO(), client.ObjectKey{Namespace: "n", Name: "c"}, &actual)
				Expect(err).NotTo(HaveOccurred())
				Expect(equality.Semantic.DeepDerivative(actual, cm)).To(BeTrue())
			})
		})
	})
})
