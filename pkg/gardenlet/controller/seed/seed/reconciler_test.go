package seed

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("UpdateAllEtcdVPATargetRefs", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		ctx = context.Background()
	})

	Context("when no relevant VPA objects exist", func() {
		var vpa *vpaautoscalingv1.VerticalPodAutoscaler

		BeforeEach(func() {
			vpa = createEtcdVPA("vpa-for-another-component-foo", "foo", "foo", "foo", false)
			Expect(fakeClient.Create(ctx, vpa)).To(Succeed())
		})

		It("should not update any non-relevant VPA objects", func() {
			Expect(updateAllEtcdVPATargetRefs(ctx, fakeClient)).To(Succeed())

			updatedVPA := &vpaautoscalingv1.VerticalPodAutoscaler{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Name: vpa.Name, Namespace: vpa.Namespace}, updatedVPA)).To(Succeed())
			Expect(updatedVPA).To(Equal(vpa))
		})
	})

	Context("when relevant VPA objects exist", func() {
		var vpas []*vpaautoscalingv1.VerticalPodAutoscaler

		BeforeEach(func() {
			vpas = []*vpaautoscalingv1.VerticalPodAutoscaler{
				createEtcdVPA("vpa-etcd-main", "foo", "etcd-vpa-main", "etcd-main", false),
				createEtcdVPA("vpa-etcd-events", "foo", "etcd-vpa-events", "etcd-events", false),
				createEtcdVPA("vpa-etcd-main", "bar", "etcd-vpa-main", "etcd-main", true),
			}
			for _, vpa := range vpas {
				Expect(fakeClient.Create(ctx, vpa)).To(Succeed())
			}
		})

		It("should update the TargetRef APIVersion and Kind to Etcd for all relevant VPAs", func() {
			Expect(updateAllEtcdVPATargetRefs(ctx, fakeClient)).To(Succeed())

			for _, vpa := range vpas {
				updatedVPA := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: vpa.Name, Namespace: vpa.Namespace}, updatedVPA)).To(Succeed())
				Expect(updatedVPA.Spec.TargetRef).To(Equal(&autoscalingv1.CrossVersionObjectReference{
					APIVersion: "druid.gardener.cloud/v1alpha1",
					Kind:       "Etcd",
					Name:       vpa.Spec.TargetRef.Name,
				}))
			}
		})
	})
})

func createEtcdVPA(name, namespace, labelValueRole, targetRefName string, shouldTargetEtcd bool) *vpaautoscalingv1.VerticalPodAutoscaler {
	vpa := &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				v1beta1constants.LabelRole: labelValueRole,
			},
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       targetRefName,
			},
		},
	}
	if shouldTargetEtcd {
		vpa.Spec.TargetRef.APIVersion = "druid.gardener.cloud/v1alpha1"
		vpa.Spec.TargetRef.Kind = "Etcd"
	}
	return vpa
}
