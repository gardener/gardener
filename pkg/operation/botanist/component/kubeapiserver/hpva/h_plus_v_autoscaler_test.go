package hpva

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test/matchers"
)

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HPlusVAutoscaler", func() {
	const (
		containerNameApiserver  = "kube-apiserver"
		containerNamePodMutator = "apiserver-proxy-pod-mutator"
		containerNameVPNSeed    = "vpn-seed"
	)
	var (
		deploymentName = "test-deployment"
		namespaceName  = "test-namespace"
		hpaName        = deploymentName + "-hpva"
		vpaName        = hpaName

		kubeClient client.Client
		ctx        = context.TODO()

		//#region Helpers
		assertObjectNotOnServer = func(obj client.Object, name string) {
			err := kubeClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: name}, obj)
			ExpectWithOffset(1, err).To(HaveOccurred())
			ExpectWithOffset(1, err).To(matchers.BeNotFoundError())
		}

		newHpva = func(isEnabled bool) (*HPlusVAutoscaler, *DesiredStateParameters) {
			return NewHPlusVAutoscaler(namespaceName, deploymentName),
				&DesiredStateParameters{
					IsEnabled:                    isEnabled,
					MinReplicaCount:              1,
					MaxReplicaCount:              4,
					ContainerNameVPNSeed:         containerNameVPNSeed,
					ContainerNameApiserver:       containerNameApiserver,
					ContainerNameProxyPodMutator: containerNamePodMutator,
				}
		}

		newHpa = func(minReplicaCount int32, maxReplicaCount int32) *autoscalingv2.HorizontalPodAutoscaler {
			lvalue300 := resource.MustParse("300")
			return &autoscalingv2.HorizontalPodAutoscaler{
				TypeMeta: metav1.TypeMeta{
					APIVersion: autoscalingv2.SchemeGroupVersion.String(),
					Kind:       "HorizontalPodAutoscaler",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            hpaName,
					Namespace:       namespaceName,
					Labels:          map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-hpa"},
					ResourceVersion: "1",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					MinReplicas: &minReplicaCount,
					MaxReplicas: maxReplicaCount,
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       deploymentName,
					},
					Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
						ScaleDown: &autoscalingv2.HPAScalingRules{
							StabilizationWindowSeconds: pointer.Int32(900),
						},
					},
					Metrics: []autoscalingv2.MetricSpec{
						{
							Type: autoscalingv2.PodsMetricSourceType,
							Pods: &autoscalingv2.PodsMetricSource{
								Metric: autoscalingv2.MetricIdentifier{Name: "shoot:apiserver_request_total:sum"},
								Target: autoscalingv2.MetricTarget{AverageValue: &lvalue300, Type: autoscalingv2.AverageValueMetricType},
							},
						},
					},
				},
			}
		}

		newVpa = func() *vpaautoscalingv1.VerticalPodAutoscaler {
			updateModeAutoAsLvalue := vpaautoscalingv1.UpdateModeAuto
			return &vpaautoscalingv1.VerticalPodAutoscaler{
				TypeMeta: metav1.TypeMeta{
					APIVersion: vpaautoscalingv1.SchemeGroupVersion.String(),
					Kind:       "VerticalPodAutoscaler",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            vpaName,
					Namespace:       namespaceName,
					Labels:          map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-vpa"},
					ResourceVersion: "1",
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       deploymentName,
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						MinReplicas: pointer.Int32(2),
						UpdateMode:  &updateModeAutoAsLvalue,
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: getVPAContainerResourcePolicies(
							containerNameApiserver, containerNamePodMutator, containerNameVPNSeed),
					},
				},
			}
		}
		//#endregion Helpers
	)

	BeforeEach(func() {
		kubeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Describe(".Reconcile()", func() {
		Context("in enabled state", func() {
			It("should deploy the correct resources to the shoot control plane", func() {
				// Arrange
				hpva, desiredState := newHpva(true)

				// Act
				Expect(hpva.Reconcile(ctx, kubeClient, desiredState)).To(Succeed())

				// Assert
				actualHpa := autoscalingv2.HorizontalPodAutoscaler{}
				Expect(kubeClient.Get(
					ctx,
					client.ObjectKey{Namespace: namespaceName, Name: hpaName},
					&actualHpa),
				).To(Succeed())
				Expect(&actualHpa).To(matchers.DeepEqual(newHpa(desiredState.MinReplicaCount, desiredState.MaxReplicaCount)))

				actualVpa := vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(kubeClient.Get(
					ctx,
					client.ObjectKey{Namespace: namespaceName, Name: vpaName},
					&actualVpa),
				).To(Succeed())
				Expect(&actualVpa).To(matchers.DeepEqual(newVpa()))
			})
			It("should not attempt to deploy proxy pod mutator, when the respective image argument is empty", func() {
				// Arrange
				hpva, desiredState := newHpva(true)
				desiredState.ContainerNameProxyPodMutator = ""

				// Act
				Expect(hpva.Reconcile(ctx, kubeClient, desiredState)).To(Succeed())

				// Assert
				actualVpa := vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(kubeClient.Get(
					ctx,
					client.ObjectKey{Namespace: namespaceName, Name: vpaName},
					&actualVpa),
				).To(Succeed())
				Expect(len(actualVpa.Spec.ResourcePolicy.ContainerPolicies)).To(Equal(2))
				Expect(actualVpa.Spec.ResourcePolicy.ContainerPolicies[0].ContainerName).To(Equal(containerNameApiserver))
				Expect(actualVpa.Spec.ResourcePolicy.ContainerPolicies[1].ContainerName).To(Equal(containerNameVPNSeed))
			})
		})
		Context("in disabled state", func() {
			It("should not deploy any resources to the shoot control plane", func() {
				// Arrange
				hpva, desiredState := newHpva(false)

				// Act
				Expect(hpva.Reconcile(ctx, kubeClient, desiredState)).To(Succeed())

				// Assert
				assertObjectNotOnServer(&autoscalingv2.HorizontalPodAutoscaler{}, hpaName)
				assertObjectNotOnServer(&vpaautoscalingv1.VerticalPodAutoscaler{}, vpaName)
			})
			It("should remove respective resources already in the shoot control plane", func() {
				// Arrange
				hpva, desiredState := newHpva(true)
				Expect(hpva.reconcileHPA(ctx, kubeClient, desiredState.MinReplicaCount, desiredState.MaxReplicaCount)).To(Succeed())
				desiredState.IsEnabled = false

				// Act
				Expect(hpva.Reconcile(ctx, kubeClient, desiredState)).To(Succeed())

				// Assert
				assertObjectNotOnServer(&autoscalingv2.HorizontalPodAutoscaler{}, hpaName)
				assertObjectNotOnServer(&vpaautoscalingv1.VerticalPodAutoscaler{}, vpaName)
			})
		})
	})
	Describe(".DeleteFromServer()", func() {
		Context("in enabled state", func() {
			It("should destroy respective resources in the shoot control plane", func() {
				// Arrange
				hpva, desiredState := newHpva(true)
				Expect(hpva.reconcileHPA(ctx, kubeClient, desiredState.MinReplicaCount, desiredState.MaxReplicaCount)).To(Succeed())

				// Act
				Expect(hpva.DeleteFromServer(ctx, kubeClient)).To(Succeed())

				// Assert
				assertObjectNotOnServer(&autoscalingv2.HorizontalPodAutoscaler{}, hpaName)
				assertObjectNotOnServer(&vpaautoscalingv1.VerticalPodAutoscaler{}, vpaName)
			})
			It("should not fail if resources are missing on the seed", func() {
				// Arrange
				hpva, _ := newHpva(true)

				// Act
				err := hpva.DeleteFromServer(ctx, kubeClient)

				// Assert
				Expect(err).To(Succeed())
				assertObjectNotOnServer(&autoscalingv2.HorizontalPodAutoscaler{}, hpaName)
				assertObjectNotOnServer(&vpaautoscalingv1.VerticalPodAutoscaler{}, vpaName)
			})
		})
		Context("in disabled state", func() {
			It("should destroy respective resources in the shoot control plane", func() {
				// Arrange
				hpva, desiredState := newHpva(true)
				Expect(hpva.reconcileHPA(ctx, kubeClient, desiredState.MinReplicaCount, desiredState.MaxReplicaCount)).To(Succeed())
				desiredState.IsEnabled = false

				// Act
				Expect(hpva.DeleteFromServer(ctx, kubeClient)).To(Succeed())

				// Assert
				assertObjectNotOnServer(&autoscalingv2.HorizontalPodAutoscaler{}, hpaName)
				assertObjectNotOnServer(&vpaautoscalingv1.VerticalPodAutoscaler{}, vpaName)
			})
		})
	})
})
