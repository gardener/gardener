// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/emicklei/go-restful"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	webhooktesting "k8s.io/apiserver/pkg/admission/plugin/webhook/testing"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/server/routes"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func TestWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook admission tests")
}

var (
	stopCh            = make(chan struct{})
	wh                *mutating.Plugin
	stopCtx, cancelFn = context.WithCancel(context.Background())
)

var _ = BeforeSuite(func(done Done) {
	o := Options{
		BindAddress:    "127.0.0.1",
		Port:           10250,
		Kubeconfig:     filepath.Join("testdata", "dummy-kubeconfig.yaml"),
		ServerCertPath: filepath.Join("testdata", "tls.crt"),
		ServerKeyPath:  filepath.Join("testdata", "tls.key"),
	}

	// create a dummy server to response with K8S version
	container := restful.NewContainer()

	routes.Version{Version: &version.Info{
		GitVersion: "v1.18.0",
		Major:      "1",
		Minor:      "18",
	}}.Install(container)

	dummyK8SServer := http.Server{
		Addr:    "127.0.0.1:11234",
		Handler: container,
	}

	go func() {
		defer GinkgoRecover()
		err := dummyK8SServer.ListenAndServe()
		Expect(err).ToNot(HaveOccurred())
	}()

	go func() {
		defer GinkgoRecover()
		o.run(stopCtx)
		Expect(dummyK8SServer.Shutdown(stopCtx)).ToNot(HaveOccurred(), "shutdown of dummy server succeeds")
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", "localhost", 10250)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()

		return nil
	}).Should(Succeed())

	webhookServer, err := url.Parse("https://localhost:10250")
	Expect(err).ToNot(HaveOccurred(), "failed to parse url")

	caCert, err := ioutil.ReadFile(filepath.Join("testdata", "ca.crt"))
	Expect(err).NotTo(HaveOccurred(), "ca.crt can be read")

	wh, err = mutating.NewMutatingWebhook(nil)
	Expect(err).ToNot(HaveOccurred(), "failed to create mutating webhook")

	client, informer := webhooktesting.NewFakeMutatingDataSource("foo", []v1.MutatingWebhook{{
		Name:                    "foo",
		NamespaceSelector:       &metav1.LabelSelector{},
		ObjectSelector:          &metav1.LabelSelector{},
		AdmissionReviewVersions: []string{"v1beta1"},
		Rules: []v1.RuleWithOperations{{
			Operations: []v1.OperationType{v1.OperationAll},
			Rule: v1.Rule{
				APIGroups:   []string{"*"},
				APIVersions: []string{"*"},
				Resources:   []string{"*/*"},
			},
		}},
		ClientConfig: v1.WebhookClientConfig{
			Service: &v1.ServiceReference{
				Name:      "webhook-test",
				Namespace: "default",
				Path:      pointer.StringPtr("/webhooks/default-pod-scheduler-name/gardener-shoot-controlplane-scheduler"),
			},
			CABundle: caCert,
		},
	}}, nil)

	wh.SetAuthenticationInfoResolverWrapper(webhooktesting.Wrapper(webhooktesting.NewAuthenticationInfoResolver(new(int32))))
	wh.SetServiceResolver(webhooktesting.NewServiceResolver(*webhookServer))
	wh.SetExternalKubeClientSet(client)
	wh.SetExternalKubeInformerFactory(informer)

	informer.Start(stopCh)
	informer.WaitForCacheSync(stopCh)

	Expect(wh.ValidateInitialization()).ToNot(HaveOccurred(), "failed to validate initialization")

	close(done)
}, 60)

var _ = AfterSuite(func() {
	cancelFn()
	close(stopCh)
})

var _ = Describe("Pod admission", func() {
	var pod, expected *corev1.Pod

	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "test",
			}, Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "test",
					},
				},
			},
		}
		expected = pod.DeepCopy()
	})

	DescribeTable("with different operations", func(op admission.Operation, expectedSchedulerName string) {
		expected.Spec.SchedulerName = expectedSchedulerName

		Expect(wh.Admit(
			context.TODO(),
			newPodAttribute(pod, op),
			webhooktesting.NewObjectInterfacesForTest(),
		)).NotTo(HaveOccurred(), "admission succeeds")
		Expect(pod).To(Equal(expected))
	},
		Entry("CREATE adds .spec.schedulerName", admission.Create, "gardener-shoot-controlplane-scheduler"),
		Entry("UPDDATE does nothing", admission.Update, ""),
		Entry("DELETE does nothing", admission.Delete, ""),
		Entry("CONNECT does nothing", admission.Connect, ""),
	)
})

func newPodAttribute(p *corev1.Pod, op admission.Operation) admission.Attributes {
	return admission.NewAttributesRecord(
		p,
		nil,
		corev1.SchemeGroupVersion.WithKind("Pod"),
		"test",
		"foo",
		corev1.SchemeGroupVersion.WithResource("pods"),
		"",
		op,
		&metav1.CreateOptions{},
		false,
		&user.DefaultInfo{
			Name:   "webhook-test",
			UID:    "webhook-test",
			Groups: nil,
			Extra:  nil,
		},
	)
}
