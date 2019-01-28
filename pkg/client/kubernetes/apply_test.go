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
	"sync"

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

	var (
		c       client.Client
		d       *fakeDiscovery
		applier *kubernetes.Applier
	)
	BeforeSuite(func() {
		c = fake.NewFakeClient()
		d = &fakeDiscovery{
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
		applier = newTestApplier(c, d)
	})
	Context("ManifestTest", func() {
		var (
			rawConfigMap = []byte(`apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  name: test-cm
  namespace: test-ns`)
		)
		Context("manifest readers testing", func() {
			It("Should read manifest correctly", func() {
				unstructuredObject, err := kubernetes.NewManifestReader(rawConfigMap).Read()
				Expect(err).NotTo(HaveOccurred())

				// Tests to ensure validity of object
				Expect(unstructuredObject.GetName()).To(Equal("test-cm"))
				Expect(unstructuredObject.GetNamespace()).To(Equal("test-ns"))
			})
		})

		It("Should read manifest and swap namespace correctly", func() {
			unstructuredObject, err := kubernetes.NewNamespaceSettingReader(kubernetes.NewManifestReader(rawConfigMap), "swap-ns").Read()
			Expect(err).NotTo(HaveOccurred())

			// Tests to ensure validity of object and namespace swap
			Expect(unstructuredObject.GetName()).To(Equal("test-cm"))
			Expect(unstructuredObject.GetNamespace()).To(Equal("swap-ns"))
		})
	})
	Context("Applier", func() {
		var rawMultipleObjects = []byte(`
apiVersion: v1
data:
  foo: bar
kind: ConfigMap
metadata:
  name: test-cm
  namespace: test-ns
---
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: test-ns
spec:
  containers:
    - name: dns
      image: dnsutils`)

		Context("#ApplyManifest", func() {
			It("should create non-existent objects", func() {
				cm := corev1.ConfigMap{
					TypeMeta:   configMapTypeMeta,
					ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "n"},
				}
				manifest := mkManifest(&cm)
				manifestReader := kubernetes.NewManifestReader(manifest)
				Expect(applier.ApplyManifest(context.TODO(), manifestReader)).To(BeNil())

				var actualCM corev1.ConfigMap
				err := c.Get(context.TODO(), client.ObjectKey{Name: "c"}, &actualCM)
				Expect(err).NotTo(HaveOccurred())
				Expect(equality.Semantic.DeepDerivative(actualCM, cm)).To(BeTrue())
			})
			It("should apply multiple objects", func() {
				manifestReader := kubernetes.NewManifestReader(rawMultipleObjects)
				Expect(applier.ApplyManifest(context.TODO(), manifestReader)).To(BeNil())

				err := c.Get(context.TODO(), client.ObjectKey{Name: "test-cm"}, &corev1.ConfigMap{})
				Expect(err).NotTo(HaveOccurred())

				err = c.Get(context.TODO(), client.ObjectKey{Name: "test-pod"}, &corev1.Pod{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create objects with namespace", func() {
				cm := corev1.ConfigMap{
					TypeMeta:   configMapTypeMeta,
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"},
				}
				manifest := mkManifest(&cm)
				manifestReader := kubernetes.NewManifestReader(manifest)
				namespaceSettingReader := kubernetes.NewNamespaceSettingReader(manifestReader, "b")
				Expect(applier.ApplyManifest(context.TODO(), namespaceSettingReader)).To(BeNil())

				var actualCMWithNamespace corev1.ConfigMap
				err := c.Get(context.TODO(), client.ObjectKey{Name: "test"}, &actualCMWithNamespace)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualCMWithNamespace.Namespace).To(Equal("b"))
			})

		})
	})
})
