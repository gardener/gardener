package extensionresources_test

import (
	"context"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensionresources"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"k8s.io/apimachinery/pkg/runtime"

	gomegatypes "github.com/onsi/gomega/types"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var dnsrecordfmt = `
{
  "apiVersion": "extensions.gardener.cloud/v1alpha1",
  "kind": "DNSRecord",
  "metadata": {
    "name": "dnsrecord-external",
	%s
    "namespace": "prjswebhooks"
  },
  "spec": {
    "type": "google-clouddns",
    "secretRef": {
      "name": %q,
      "namespace": "prjswebhooks"
    },
    "name": "api.gcp.foobar.shoot.example.com",
    "recordType": %q,
    "values": [
      "1.2.3.4"
    ]
  }
}
`

var _ = Describe("handler", func() {
	Describe("#validateExternalResources", func() {
		var (
			ctx    context.Context
			logger logr.Logger

			decoder *admission.Decoder
			handler admission.Handler

			request *admission.Request

			ctrl *gomock.Controller
			c    *mockclient.MockClient

			resources []dummyResource
		)

		BeforeEach(func() {
			ctx = context.TODO()
			logger = logzap.New(logzap.WriteTo(GinkgoWriter))

			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			var err error
			decoder, err = admission.NewDecoder(kubernetes.SeedScheme)
			Expect(err).NotTo(HaveOccurred())

			handler = extensionresources.New(logger)
			Expect(inject.APIReaderInto(c, handler)).To(BeTrue())
			Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

			request = nil
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("ignored requests", func() {
			It("should ignore DELETE operations", func() {
				request = newRequest(dummyResource{operation: admissionv1.Delete})
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("operation is not CREATE or UPDATE"))
			})
			It("should ignore CONNECT operations", func() {
				request = newRequest(dummyResource{operation: admissionv1.Connect})
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("operation is not CREATE or UPDATE"))
			})
		})

		Context("create resources", func() {

			var (
				dnsrecordObj = fmt.Sprintf(dnsrecordfmt, "", "dnsrecord-external", "A")
			)

			BeforeEach(func() {
				resources = []dummyResource{
					//				 "backupbuckets"
					//				 "backupentries"
					//				 "containerruntimes"
					//				 "controlplanes"
					//				 "dnsrecords"
					{
						operation: admissionv1.Create,
						kind:      extensionsv1alpha1.DNSRecordResource,
						obj:       runtime.RawExtension{Raw: []byte(dnsrecordObj)},
					},
					//				 "extensions"
					//				 "infrastructures"
					//				 "networks"
					//				 "operatingsystemconfigs"
					//				 "workers"
				}
			})

			It("should create successfully the resources", func() {
				for _, r := range resources {
					request = newRequest(r)
					expectAllowed(handler.Handle(ctx, *request), ContainSubstring(""))
				}
			})
		})

		Context("update resources", func() {
			BeforeEach(func() {
				var (
					dnsrecordObj    = fmt.Sprintf(dnsrecordfmt, `"resourceVersion": "2",`, "dnsrecord-external", "A")
					dnsrecordOldObj = fmt.Sprintf(dnsrecordfmt, `"resourceVersion": "1",`, "dnsrecord-external-2", "A")
				)

				resources = []dummyResource{
					//				 "backupbuckets"
					//				 "backupentries"
					//				 "containerruntimes"
					//				 "controlplanes"
					{ //	"dnsrecords"
						operation: admissionv1.Update,
						kind:      extensionsv1alpha1.DNSRecordResource,
						obj:       runtime.RawExtension{Raw: []byte(dnsrecordObj)},
						oldobj:    runtime.RawExtension{Raw: []byte(dnsrecordOldObj)},
					},
					//				 "extensions"
					//				 "infrastructures"
					//				 "networks"
					//				 "operatingsystemconfigs"
					//				 "workers"
				}
			})

			It("should update successfully the resources", func() {
				for _, r := range resources {
					request = newRequest(r)
					expectAllowed(handler.Handle(ctx, *request), ContainSubstring(""))
				}
			})
		})
	})
})

type dummyResource struct {
	operation admissionv1.Operation
	kind      string
	obj       runtime.RawExtension
	oldobj    runtime.RawExtension
}

func newRequest(dr dummyResource) *admission.Request {
	r := new(admission.Request)
	r.Operation = dr.operation
	r.Kind.Kind = dr.kind
	r.Object = dr.obj
	r.OldObject = dr.oldobj
	return r
}

func expectAllowed(r admission.Response, reason gomegatypes.GomegaMatcher) {
	Expect(r.Allowed).To(BeTrue())
	Expect(string(r.Result.Reason)).To(reason)
}
