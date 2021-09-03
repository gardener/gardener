// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensionresources_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensionresources"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	apiversion  = "extensions.gardener.cloud/v1alpha1"
	defaultSpec = extensionsv1alpha1.DefaultSpec{Type: "gcp"}
	objectMeta  = metav1.ObjectMeta{Name: "entity-external", Namespace: "prjswebhooks"}
	secretRef   = corev1.SecretReference{Name: "secret-external", Namespace: "prjswebhooks"}

	backupBucket = &extensionsv1alpha1.BackupBucket{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.BackupBucketResource,
			APIVersion: apiversion,
		},
		ObjectMeta: metav1.ObjectMeta{Name: "backupbucket-external"},
		Spec: extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: defaultSpec,
			Region:      "europe-west-1",
			SecretRef:   secretRef,
		},
	}

	backupEntry = &extensionsv1alpha1.BackupEntry{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.BackupEntryResource,
			APIVersion: apiversion,
		},
		ObjectMeta: metav1.ObjectMeta{Name: "backupentry-external"},
		Spec: extensionsv1alpha1.BackupEntrySpec{
			DefaultSpec: defaultSpec,
			Region:      "europe-west-1",
			BucketName:  "cloud--gcp--fg2d6",
			SecretRef:   secretRef,
		},
	}

	bastion = &extensionsv1alpha1.Bastion{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.BastionResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.BastionSpec{
			DefaultSpec: defaultSpec,
			UserData:    []byte("data"),
			Ingress: []extensionsv1alpha1.BastionIngressPolicy{
				{
					IPBlock: networkingv1.IPBlock{
						CIDR: "1.2.3.4/32",
					},
				},
			},
		},
	}

	containerRuntime = &extensionsv1alpha1.ContainerRuntime{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.ContainerRuntimeResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
	}

	controlPlane = &extensionsv1alpha1.ControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.ControlPlaneResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: defaultSpec,
			SecretRef:   secretRef,
			Region:      "europe-west-1",
		},
	}

	dnsrecord = &extensionsv1alpha1.DNSRecord{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.DNSRecordResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.DNSRecordSpec{
			DefaultSpec: defaultSpec,
			SecretRef:   secretRef,
			Name:        "api.gcp.foobar.shoot.example.com",
			RecordType:  "A",
			Values:      []string{"1.2.3.4"},
		},
	}

	extension = &extensionsv1alpha1.Extension{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.ExtensionResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.ExtensionSpec{
			DefaultSpec: defaultSpec,
		},
	}

	infrastructure = &extensionsv1alpha1.Infrastructure{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.InfrastructureResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: defaultSpec,
			SecretRef:   secretRef,
			Region:      "europe-west-1",
		},
	}

	network = &extensionsv1alpha1.Network{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.NetworkResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.NetworkSpec{
			DefaultSpec: defaultSpec,
			PodCIDR:     "100.96.0.0/11",
			ServiceCIDR: "100.64.0.0/13",
		},
	}

	osc = &extensionsv1alpha1.OperatingSystemConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.OperatingSystemConfigResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
			Purpose:     extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
			DefaultSpec: defaultSpec,
		},
	}

	worker = &extensionsv1alpha1.Worker{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionsv1alpha1.WorkerResource,
			APIVersion: apiversion,
		},
		ObjectMeta: objectMeta,
		Spec: extensionsv1alpha1.WorkerSpec{
			DefaultSpec: defaultSpec,
			Region:      "europe-west-1",
			SecretRef:   secretRef,
		},
	}
)

var _ = Describe("handler", func() {
	Describe("#validateExternalResources", func() {
		var (
			ctx    context.Context
			logger logr.Logger

			decoder *admission.Decoder
			handler admission.Handler

			request *admission.Request

			ctrl *gomock.Controller
		)

		BeforeEach(func() {
			ctx = context.TODO()
			logger = logzap.New(logzap.WriteTo(GinkgoWriter))

			ctrl = gomock.NewController(GinkgoT())

			var err error
			decoder, err = admission.NewDecoder(kubernetes.SeedScheme)
			Expect(err).NotTo(HaveOccurred())

			handler = extensionresources.New(logger, false)
			Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

			request = nil
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("ignored requests", func() {
			It("should ignore DELETE operations", func() {
				request = newRequest(admissionv1.Delete, "dnsrecords", nil, nil)
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("operation is not CREATE or UPDATE"))
			})

			It("should ignore CONNECT operations", func() {
				request = newRequest(admissionv1.Connect, "dnsrecords", nil, nil)
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("operation is not CREATE or UPDATE"))
			})

			It("should ignore different resources", func() {
				request = newRequest(admissionv1.Create, "customresourcedefinitions", nil, nil)
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("validation not found for the given resource"))
			})
		})

		Context("create resources", func() {
			DescribeTable("should create successfully the resource",
				func(kind string, obj runtime.Object) {
					request = newRequest(admissionv1.Create, kind, obj, nil)
					expectAllowed(handler.Handle(ctx, *request), ContainSubstring("validation successful"), resourceToId(request.Resource))
				},

				Entry("for backupbuckets", "backupbuckets", backupBucket),
				Entry("for backupentries", "backupentries", backupEntry),
				Entry("for bastions", "bastions", bastion),
				Entry("for containerruntime", "containerruntimes", containerRuntime),
				Entry("for controlplanes", "controlplanes", controlPlane),
				Entry("for dnsrecords", "dnsrecords", dnsrecord),
				Entry("for extensions", "extensions", extension),
				Entry("for infrastructures", "infrastructures", infrastructure),
				Entry("for networks", "networks", network),
				Entry("for operatingsystemconfigs", "operatingsystemconfigs", osc),
				Entry("for workers", "workers", worker),
			)

			It("decoding of new dns record should fail", func() {
				request = newRequest(admissionv1.Create, "dnsrecords", dnsrecord, nil)

				// intentionally break JSON validity
				obj := string(request.Object.Raw)
				request.Object.Raw = []byte(strings.Replace(obj, "{", "", 1))

				expectErrored(handler.Handle(ctx, *request), ContainSubstring("could not decode object"), BeEquivalentTo(http.StatusUnprocessableEntity))
			})
		})

		Context("update resources", func() {
			DescribeTable("should update successfully the resource",
				func(kind string, new, old runtime.Object) {
					request = newRequest(admissionv1.Update, kind, new, old)
					expectAllowed(handler.Handle(ctx, *request), ContainSubstring("validation successful"), resourceToId(request.Resource))
				},

				Entry("for backupbuckets", "backupbuckets", newBackupBucket("2", "backupbucket-external", ""), newBackupBucket("2", "backupbucket-external-2", "")),
				Entry("for backupentries", "backupentries", newBackupEntry("2", "backupentry-external-2", ""), newBackupEntry("1", "", "")),
				Entry("for bastions", "bastions", newBastion("2", "1.1.1.1/16", ""), newBastion("1", "", "")),
				// TODO: Fix this with #4561
				Entry("for containerruntime", "containerruntimes", newContainerRuntime("2"), newContainerRuntime("1")),
				Entry("for controlplanes", "controlplanes", newControlPlane("2", "cloudprovider", ""), newControlPlane("1", "", "")),
				Entry("for dnsrecords", "dnsrecords", newDNSRecord("2", "dnsrecord-external", ""), newDNSRecord("1", "", "")),
				Entry("for extensions", "extensions", newExtension("2", "", &runtime.RawExtension{}), newExtension("1", "", nil)),
				Entry("for infrastructures", "infrastructures", newInfrastructure("2", "infrastructure-external", ""), newInfrastructure("1", "", "")),
				Entry("for networks", "networks", newNetwork("2", ""), newNetwork("1", "")),
				Entry("for operatingsystemconfigs", "operatingsystemconfigs", newOSC("2", "path/to/file", ""), newOSC("1", "", "")),
				Entry("for workers", "workers", newWorker("2", "workers-external", ""), newWorker("1", "", "")),
			)

			DescribeTable("update should fail",
				func(kind string, wrong, old runtime.Object) {
					request = newRequest(admissionv1.Update, kind, wrong, old)
					expectDenied(handler.Handle(ctx, *request), ContainSubstring(""), resourceToId(request.Resource))
				},
				Entry("for backupbuckets", "backupbuckets", newBackupBucket("2", "backupbucket-external", "azure"), newBackupBucket("1", "", "")),
				Entry("for backupentries", "backupentries", newBackupEntry("2", "backupentry-external", "azure"), newBackupEntry("1", "", "")),
				Entry("for bastions", "bastions", newBastion("2", "1.1.1.1/16", "azure"), newBastion("1", "", "")),
				// TODO: Introduce entry for ContainerRuntime with #4561
				Entry("for controlplanes", "controlplanes", newControlPlane("2", "cloudprovider", "azure"), newControlPlane("1", "", "")),
				Entry("for dnsrecords", "dnsrecords", newDNSRecord("2", "dnsrecord-external", "TXT"), newDNSRecord("1", "", "")),
				Entry("for extensions", "extensions", newExtension("2", "azure", nil), newExtension("1", "", nil)),
				Entry("for infrastructures", "infrastructures", newInfrastructure("2", "infrastructure-external", "azure"), newInfrastructure("2", "infrastructure-external", "")),
				Entry("for networks", "networks", newNetwork("2", "1.1.1.1/16"), newNetwork("1", "")),
				Entry("for operatingsystemconfigs", "operatingsystemconfigs", newOSC("2", "", "azure"), newOSC("2", "", "")),
				Entry("for workers", "workers", newWorker("2", "", "azure"), newWorker("1", "", "")),
			)
		})
	})
})

func newRequest(operation admissionv1.Operation, kind string, obj, oldobj runtime.Object) *admission.Request {
	r := new(admission.Request)

	r.Operation = operation
	r.Resource = metav1.GroupVersionResource{
		Group:    extensionsv1alpha1.SchemeGroupVersion.Group,
		Version:  extensionsv1alpha1.SchemeGroupVersion.Version,
		Resource: kind,
	}
	r.Object = runtime.RawExtension{Raw: marshalObject(obj)}
	r.OldObject = runtime.RawExtension{Raw: marshalObject(oldobj)}

	return r
}

func marshalObject(obj runtime.Object) []byte {
	o, err := json.Marshal(obj)
	if err != nil {
		return nil
	}

	return o
}

func expectAllowed(r admission.Response, reason gomegatypes.GomegaMatcher, description ...interface{}) {
	Expect(string(r.Result.Reason)).To(reason, description...)
	Expect(r.Allowed).To(BeTrue(), description...)
}

func expectErrored(r admission.Response, reason, code gomegatypes.GomegaMatcher, description ...interface{}) {
	Expect(r.Result.Message).To(reason, description...)
	Expect(r.Result.Code).To(code, description...)
	Expect(r.Allowed).To(BeFalse(), description...)
}

func expectDenied(r admission.Response, reason gomegatypes.GomegaMatcher, description ...interface{}) {
	Expect(r.Result.Message).To(reason, description...)
	Expect(r.Allowed).To(BeFalse(), description...)
}

func resourceToId(resource metav1.GroupVersionResource) string {
	return fmt.Sprintf("%s/%s/%s", resource.Group, resource.Version, resource.Resource)
}

func newBackupBucket(resourcesVersion, secretRefName, specType string) runtime.Object {
	b := backupBucket.DeepCopy()

	if resourcesVersion != "" {
		b.ResourceVersion = resourcesVersion
	}
	if secretRefName != "" {
		b.Spec.SecretRef.Name = secretRefName
	}
	if specType != "" {
		b.Spec.Type = specType
	}

	return b
}

func newBackupEntry(resourcesVersion, secretRefName, specType string) runtime.Object {
	b := backupEntry.DeepCopy()

	if resourcesVersion != "" {
		b.ResourceVersion = resourcesVersion
	}
	if secretRefName != "" {
		b.Spec.SecretRef.Name = secretRefName
	}
	if specType != "" {
		b.Spec.Type = specType
	}

	return b
}

func newBastion(resourcesVersion, cidr, specType string) runtime.Object {
	b := bastion.DeepCopy()

	if resourcesVersion != "" {
		b.ResourceVersion = resourcesVersion
	}
	if cidr != "" {
		b.Spec.Ingress[0].IPBlock.CIDR = cidr
	}
	if specType != "" {
		b.Spec.Type = specType
	}

	return b
}

func newContainerRuntime(resourcesVersion string) runtime.Object {
	c := containerRuntime.DeepCopy()

	if resourcesVersion != "" {
		c.ResourceVersion = resourcesVersion
	}

	return c
}

func newControlPlane(resourcesVersion, secretRefName, specType string) runtime.Object {
	b := controlPlane.DeepCopy()

	if resourcesVersion != "" {
		b.ResourceVersion = resourcesVersion
	}
	if secretRefName != "" {
		b.Spec.SecretRef.Name = secretRefName
	}
	if specType != "" {
		b.Spec.Type = specType
	}

	return b
}

func newDNSRecord(resourcesVersion, secretRefName, recordType string) runtime.Object {
	d := dnsrecord.DeepCopy()

	if resourcesVersion != "" {
		d.ResourceVersion = resourcesVersion
	}
	if secretRefName != "" {
		d.Spec.SecretRef.Name = secretRefName
	}
	if recordType != "" {
		d.Spec.RecordType = extensionsv1alpha1.DNSRecordType(recordType)
	}

	return d
}

func newExtension(resourcesVersion, specType string, config *runtime.RawExtension) runtime.Object {
	e := extension.DeepCopy()

	if resourcesVersion != "" {
		e.ResourceVersion = resourcesVersion
	}
	if specType != "" {
		e.Spec.Type = specType
	}
	if config != nil {
		e.Spec.ProviderConfig = config
	}

	return e
}

func newInfrastructure(resourcesVersion, secretRefName, specType string) runtime.Object {
	i := infrastructure.DeepCopy()

	if resourcesVersion != "" {
		i.ResourceVersion = resourcesVersion
	}
	if secretRefName != "infrastructure-external" {
		i.Spec.SecretRef.Name = secretRefName
	}
	if specType != "" {
		i.Spec.Type = specType
	}

	return i
}

func newNetwork(resourcesVersion, podCIDR string) runtime.Object {
	n := network.DeepCopy()

	if resourcesVersion != "" {
		n.ResourceVersion = resourcesVersion
	}
	if podCIDR != "" {
		n.Spec.PodCIDR = podCIDR
	}

	return n
}

func newOSC(resourcesVersion, path, specType string) runtime.Object {
	o := osc.DeepCopy()

	if resourcesVersion != "" {
		o.ResourceVersion = resourcesVersion
	}
	if path != "" {
		o.Spec.ReloadConfigFilePath = pointer.String(path)
	}
	if specType != "" {
		o.Spec.Type = specType
	}

	return o
}

func newWorker(resourcesVersion, secretRefName, specType string) runtime.Object {
	w := worker.DeepCopy()

	if resourcesVersion != "" {
		w.ResourceVersion = resourcesVersion
	}
	if secretRefName != "" {
		w.Spec.SecretRef.Name = secretRefName
	}
	if specType != "" {
		w.Spec.Type = specType
	}

	return w
}
