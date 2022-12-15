/*
Package hpva implements the "HPlusVAutoscaler" - an autoscaling setup for kube-apiserver comprising an independently
driven horizontal and vertical pod autoscalers.

The HPA is driven by an application-specific load metric, based on the rate of requests made to the server. The goal of
HPA is to determine a rough value for the minimal number of replicas guaranteed to suffice for processing the load. That
rough estimate comes with a substantial safety margin which is offset by VPA shrinking the replicas as necessary (see below).

The VPA element is a typical VPA setup acting on both CPU and memory. The goal of VPA is to vertically adjust the
replicas provided based on HPA's rough estimate, to a scale that best matches the actual need for compute power.
*/
package hpva

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DesiredStateParameters contains all configurable options of the HPlusVAutoscaler's desired state
type DesiredStateParameters struct {
	ContainerNameProxyPodMutator string // Empty string indicates that pod mutator is disabled
	ContainerNameApiserver       string
	IsEnabled                    bool
	MaxReplicaCount              int32
	MinReplicaCount              int32
}

// HPlusVAutoscaler implements an autoscaling setup for kube-apiserver comprising an independently driven horizontal and
// vertical pod autoscalers. For further overview of the autoscaling behavior, see package hpva.
//
// The underlying implementation is an arrangement of k8s resources deployed as part of the target shoot's control plane.
// An HPlusVAutoscaler object itself is stateless. As far as state is concerned, it is nothing more than a handle,
// pointing to the server-side setup.
type HPlusVAutoscaler struct {
	deploymentNameApiserver string // Also used as name for the underlying HPA and VPA resources
	namespaceName           string
}

// NewHPlusVAutoscaler creates a local handle object, pointed at a server-side HPlusVAutoscaler instance of interest (either
// already existing, or desired). The resulting object can be used to manipulate the server-side setup.
func NewHPlusVAutoscaler(namespaceName string, deploymentNameApiserver string) *HPlusVAutoscaler {
	return &HPlusVAutoscaler{
		namespaceName:           namespaceName,
		deploymentNameApiserver: deploymentNameApiserver,
	}
}

// DeleteFromServer removes all HPlusVAutoscaler artefacts from the shoot's control plane.
// The kubeClient parameter specifies a connection to the server hosting said control plane.
func (hpva *HPlusVAutoscaler) DeleteFromServer(ctx context.Context, kubeClient client.Client) error {
	baseErrorMessage :=
		fmt.Sprintf("An error occurred while deleting HPlusVAutoscaler '%s' in namespace '%s'",
			hpva.deploymentNameApiserver,
			hpva.namespaceName)

	if err := client.IgnoreNotFound(kutil.DeleteObject(ctx, kubeClient, hpva.makeHPA())); err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to delete the HPA which is part of the HPlusVAutoscaler from the server. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	if err := client.IgnoreNotFound(kutil.DeleteObject(ctx, kubeClient, hpva.makeVPA())); err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to delete the VPA which is part of the HPlusVAutoscaler from the server. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	return nil
}

// Reconcile brings the server-side HPlusVAutoscaler setup in compliance with the desired state specified by the
// operation's parameters.
// The kubeClient parameter specifies a connection to the server hosting said control plane.
// The 'parameters' parameter specifies the desired state that is to be applied upon the server-side autoscaler setup.
func (hpva *HPlusVAutoscaler) Reconcile(
	ctx context.Context, kubeClient client.Client, parameters *DesiredStateParameters) error {

	baseErrorMessage :=
		fmt.Sprintf("An error occurred while reconciling HPlusVAutoscaler '%s' in namespace '%s'",
			hpva.deploymentNameApiserver,
			hpva.namespaceName)

	if !parameters.IsEnabled {
		if err := hpva.DeleteFromServer(ctx, kubeClient); err != nil {
			return fmt.Errorf(baseErrorMessage+
				" - failed to bring the HPlusVAutoscaler on the server to a disabled state. "+
				"The error message reported by the underlying operation follows: %w",
				err)
		}
		return nil
	}

	err := hpva.reconcileHPA(ctx, kubeClient, parameters.MinReplicaCount, parameters.MaxReplicaCount)
	if err != nil {
		return fmt.Errorf(baseErrorMessage+
			" - failed to reconcile the HPA which is part of the HPlusVAutoscaler on the server. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	if err := hpva.reconcileVPA(
		ctx,
		kubeClient,
		parameters.ContainerNameProxyPodMutator,
		parameters.ContainerNameApiserver); err != nil {

		return fmt.Errorf(baseErrorMessage+
			" - failed to reconcile the VPA which is part of the HPlusVAutoscaler from the server. "+
			"The error message reported by the underlying operation follows: %w",
			err)
	}

	return nil
}

//#region Private implementation

// Returns the name of HPlusVAutoscaler's server-side HPA
func (hpva *HPlusVAutoscaler) GetHPAName() string {
	return hpva.deploymentNameApiserver + "-hpva"
}

// Returns the name of HPlusVAutoscaler's server-side VPA
func (hpva *HPlusVAutoscaler) GetVPAName() string {
	return hpva.GetHPAName()
}

// Returns an empty HPA object pointing to the server-side HPA, which is part of this HPlusVAutoscaler
func (hpva *HPlusVAutoscaler) makeHPA() *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: hpva.GetHPAName(), Namespace: hpva.namespaceName},
	}
}

// Returns an empty VPA object pointing to the server-side VPA, which is part of this HPlusVAutoscaler
func (hpva *HPlusVAutoscaler) makeVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: hpva.GetVPAName(), Namespace: hpva.namespaceName},
	}
}

// Reconciles the HPA resource which is part of the HPlusVAutoscaler
func (hpva *HPlusVAutoscaler) reconcileHPA(
	ctx context.Context, kubeClient client.Client, minReplicaCount int32, maxReplicaCount int32) error {

	hpa := hpva.makeHPA()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, kubeClient, hpa, func() error {
		hpa.Spec.ScaleTargetRef = autoscalingv2.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       hpva.deploymentNameApiserver,
		}
		hpa.Spec.Behavior = &autoscalingv2.HorizontalPodAutoscalerBehavior{
			ScaleDown: &autoscalingv2.HPAScalingRules{
				StabilizationWindowSeconds: pointer.Int32(900),
			},
		}

		lvalue300 := resource.MustParse("300")
		hpaMetrics := []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.PodsMetricSourceType,
				Pods: &autoscalingv2.PodsMetricSource{
					Metric: autoscalingv2.MetricIdentifier{Name: "shoot:apiserver_request_total:sum"},
					Target: autoscalingv2.MetricTarget{AverageValue: &lvalue300, Type: autoscalingv2.AverageValueMetricType},
				},
			},
		}
		hpa.Spec.Metrics = hpaMetrics
		hpa.Spec.MinReplicas = &minReplicaCount
		hpa.Spec.MaxReplicas = maxReplicaCount
		hpa.ObjectMeta.Labels = map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-hpa"}

		return nil
	})

	if err != nil {
		return fmt.Errorf("An error occurred while reconciling the '%s' HPA which is part of the HPlusVAutoscaler "+
			"in namespace '%s' - failed to apply the desired configuration values to the server-side object. "+
			"The error message reported by the underlying operation follows: %w",
			hpva.deploymentNameApiserver,
			hpva.namespaceName,
			err)
	}

	return nil
}

// Reconciles the VPA resource which part of the HPlusVAutoscaler
// The containerNameAPIServerProxyPodMutator parameter is empty when the mutator is disabled
func (hpva *HPlusVAutoscaler) reconcileVPA(
	ctx context.Context,
	kubeClient client.Client,
	containerNameProxyPodMutator string,
	containerNameApiserver string) error {

	vpa := hpva.makeVPA()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, kubeClient, vpa, func() error {
		vpa.Spec.Recommenders = nil
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       hpva.deploymentNameApiserver,
		}
		updateModeAutoAsLvalue := vpaautoscalingv1.UpdateModeAuto
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
			UpdateMode: &updateModeAutoAsLvalue,
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: getVPAContainerResourcePolicies(
				containerNameApiserver, containerNameProxyPodMutator),
		}
		vpa.ObjectMeta.Labels = map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-vpa"}

		return nil
	})

	if err != nil {
		return fmt.Errorf("An error occurred while reconciling the '%s' VPA which is part of the HPlusVAutoscaler "+
			"in namespace '%s' - failed to apply the desired configuration values to the server-side object. "+
			"The error message reported by the underlying operation follows: %w",
			hpva.deploymentNameApiserver,
			hpva.namespaceName,
			err)
	}

	return nil
}

// The containerNameAPIServerProxyPodMutator parameter must be empty when the mutator is disabled
func getVPAContainerResourcePolicies(
	containerNameApiserver string,
	containerNameProxyPodMutator string) []vpaautoscalingv1.ContainerResourcePolicy {

	containerPolicyAutoAsLvalue := vpaautoscalingv1.ContainerScalingModeAuto
	containerPolicyOffAsLvalue := vpaautoscalingv1.ContainerScalingModeOff
	controlledValuesRequestsOnlyAsLvalue := vpaautoscalingv1.ContainerControlledValuesRequestsOnly

	result := []vpaautoscalingv1.ContainerResourcePolicy{
		{
			ContainerName: containerNameApiserver,
			Mode:          &containerPolicyAutoAsLvalue,
			MinAllowed: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("300m"),
				corev1.ResourceMemory: resource.MustParse("400M"),
			},
			MaxAllowed: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("25G"),
			},
			ControlledValues: &controlledValuesRequestsOnlyAsLvalue,
		},
	}

	if containerNameProxyPodMutator != "" {
		result = append(result, vpaautoscalingv1.ContainerResourcePolicy{
			ContainerName:    containerNameProxyPodMutator,
			Mode:             &containerPolicyOffAsLvalue,
			ControlledValues: &controlledValuesRequestsOnlyAsLvalue,
		})
	}

	return result
}

//#endregion Private implementation
