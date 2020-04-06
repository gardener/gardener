// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicies

import (
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ = Describe("Types", func() {

	Context("#NewPod", func() {
		It("should return correct Pod", func() {
			expected := Pod{Name: "test", Labels: map[string]string{"foo": "bar"}, ShootVersionConstraint: "> 1.0"}
			result := NewPod("test", map[string]string{"foo": "bar"}, "> 1.0")
			Expect(result).To(Equal(expected))
		})
	})

	Context("HostRule", func() {
		var (
			host = Host{HostName: "foo.bar", Port: 1234, Description: "Some service"}
		)
		It("should return correct response when allowed", func() {
			hr := HostRule{Host: host, Allowed: true}
			Expect(hr.ToString()).To(Equal(`should allow connection to "Some service" foo.bar:1234`))
		})

		It("should return correct response when not allowed", func() {
			hr := HostRule{Host: host, Allowed: false}
			Expect(hr.ToString()).To(Equal(`should block connection to "Some service" foo.bar:1234`))
		})
	})

	Context("PodRule", func() {
		var (
			pod = TargetPod{Pod: NewPod("test", map[string]string{"foo": "bar"}, "> 1.0"), Port: Port{Port: 1234}}
		)
		It("should return correct response when allowed", func() {
			pr := PodRule{TargetPod: pod, Allowed: true}
			Expect(pr.ToString()).To(Equal(`should allow connection to Pod "test" at port 1234`))
		})

		It("should return correct response when not allowed", func() {
			pr := PodRule{TargetPod: pod, Allowed: false}
			Expect(pr.ToString()).To(Equal(`should block connection to Pod "test" at port 1234`))
		})
	})

	Context("Pod", func() {

		Context("#CheckVersion", func() {

			It("should return true when no Pod version constraint is provided", func() {
				p := Pod{}
				Expect(p.CheckVersion(nil)).To(BeTrue())
			})

			It("should panic when Pod version constraint is invalid", func() {
				Expect(func() {
					p := Pod{ShootVersionConstraint: "something invalid"}
					p.CheckVersion(nil)
				}).To(Panic())
			})

			It("should panic when Shoot kubernetes version is invalid", func() {
				Expect(func() {
					shoot := &v1beta1.Shoot{
						Spec: v1beta1.ShootSpec{
							Kubernetes: v1beta1.Kubernetes{
								Version: "something invalid",
							},
						},
					}
					p := Pod{ShootVersionConstraint: "> 1.0"}
					p.CheckVersion(shoot)
				}).To(Panic())
			})

			It("should return false when Pod versions is greater than Shoot version", func() {
				shoot := &v1beta1.Shoot{
					Spec: v1beta1.ShootSpec{
						Kubernetes: v1beta1.Kubernetes{
							Version: "0.9",
						},
					},
				}
				p := Pod{ShootVersionConstraint: "> 1.0"}
				Expect(p.CheckVersion(shoot)).To(BeFalse())
			})
			It("should return true when Pod versions matches than Shoot version", func() {
				shoot := &v1beta1.Shoot{
					Spec: v1beta1.ShootSpec{
						Kubernetes: v1beta1.Kubernetes{
							Version: "1.1",
						},
					},
				}
				p := Pod{ShootVersionConstraint: "> 1.0"}
				Expect(p.CheckVersion(shoot)).To(BeTrue())
			})
		})
		Context("#Selector", func() {

			It("should convert label to selector", func() {
				p := Pod{Labels: map[string]string{"foo": "bar"}}
				selector, err := labels.Parse("foo=bar")
				Expect(err).NotTo(HaveOccurred())
				Expect(p.Selector()).To(Equal(selector))
			})
		})

		Context("#CheckSeedCluster", func() {
			It("should be true when no SeedClusterConstraints is set", func() {
				p := Pod{}
				Expect(p.CheckSeedCluster("dummy")).To(BeTrue())
			})
			It("should be false when no SeedClusterConstraints is matched", func() {
				p := Pod{SeedClusterConstraints: sets.NewString("foo", "bar")}
				Expect(p.CheckSeedCluster("dummy")).To(BeFalse())
			})
			It("should be false when SeedClusterConstraints is matched", func() {
				p := Pod{SeedClusterConstraints: sets.NewString("foo", "matched")}
				Expect(p.CheckSeedCluster("matched")).To(BeTrue())
			})
		})
	})

	Context("SourcePod", func() {

		var (
			pod Pod
		)

		BeforeEach(func() {
			pod = NewPod("foo", map[string]string{"foo": "bar"}, "> 1.0")
		})

		Context("#AsTargetPods", func() {

			It("should return TargetPod for each Port", func() {
				tp := SourcePod{Pod: pod, Ports: []Port{{Port: 8080}, {Port: 8081}}}
				expectedOne := &TargetPod{pod, Port{Port: 8080}}
				expectedTwo := &TargetPod{pod, Port{Port: 8081}}
				Expect(tp.AsTargetPods()).To(ConsistOf(expectedOne, expectedTwo))
			})
			It("should return dummy TargetPod for when no POd is provided", func() {
				tp := SourcePod{Pod: pod, Ports: []Port{}}
				dummy := &TargetPod{pod, Port{Port: 8080, Name: "dummy"}}
				Expect(tp.AsTargetPods()).To(ConsistOf(dummy))
			})
		})

		Context("#FromPort", func() {

			It("should return TargetPod existingPort", func() {
				tp := SourcePod{Pod: pod, Ports: []Port{{8080, "port-1"}, {8081, "port-2"}}}
				expectedTwo := &TargetPod{pod, Port{8081, "port-2"}}
				Expect(tp.FromPort("port-2")).To(Equal(expectedTwo))
			})

			It("should panic when port with provided name doesn't exist", func() {
				tp := SourcePod{Pod: pod, Ports: []Port{{8080, "port-1"}, {8081, "port-2"}}}

				Expect(func() {
					tp.FromPort("some-dummy-port")
				}).To(Panic())
			})
		})

		Context("#DummyPort", func() {

			It("should return dummy TargetPod ", func() {
				tp := SourcePod{Pod: pod}
				expectedTwo := &TargetPod{pod, Port{8080, "dummy"}}
				Expect(tp.DummyPort()).To(Equal(expectedTwo))
			})

			It("should panic when Pod has any Ports", func() {
				tp := SourcePod{Pod: pod, Ports: []Port{{8080, "port-1"}}}

				Expect(func() {
					tp.DummyPort()
				}).To(Panic())
			})
		})
	})

	Context("#NewNamespacedSourcePod", func() {

		It("should return correct NamespacedSourcePod", func() {
			pod := NewPod("foo", map[string]string{"foo": "bar"}, "> 1.0")
			sp := &SourcePod{Pod: pod, Ports: NewSinglePort(1234)}
			expected := &NamespacedSourcePod{sp, "bar"}
			Expect(NewNamespacedSourcePod(sp, "bar")).To(Equal(expected))
		})
	})
	Context("#NewNamespacedTargetPod", func() {

		It("should return correct NamespacedTargetPod", func() {
			pod := NewPod("foo", map[string]string{"foo": "bar"}, "> 1.0")
			sp := &TargetPod{Pod: pod, Port: Port{Port: 1234}}
			expected := &NamespacedTargetPod{sp, "bar"}
			Expect(NewNamespacedTargetPod(sp, "bar")).To(Equal(expected))
		})
	})

})
