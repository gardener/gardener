package seed

import (
	"context"
	"fmt"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// updateAllEtcdVPATargetRefs updates the target references of all VPAs for etcd components across all namespaces.
// TODO(shreyas-s-rao): Remove this function and `updateEtcdVPATargetRefs()` in v1.123.0.
func updateAllEtcdVPATargetRefs(ctx context.Context, c client.Client) error {
	roles := []string{
		v1beta1constants.ETCDRoleMain,
		v1beta1constants.ETCDRoleEvents,
	}

	for _, role := range roles {
		if err := updateEtcdVPATargetRefs(ctx, c, role); err != nil {
			return fmt.Errorf("failed to update VPA target ref for role %s: %w", role, err)
		}
	}
	return nil
}

// updateEtcdVPATargetRefs updates the target references of all VPAs for the specified etcd role.
func updateEtcdVPATargetRefs(ctx context.Context, c client.Client, role string) error {
	labelSelector := client.MatchingLabels{
		v1beta1constants.LabelRole: "etcd-vpa-" + role,
	}

	vpaList := &vpaautoscalingv1.VerticalPodAutoscalerList{}
	if err := c.List(ctx, vpaList, labelSelector); err != nil {
		return fmt.Errorf("failed to list VPAs: %w", err)
	}

	fns := make([]flow.TaskFn, 0, len(vpaList.Items))

	for _, vpa := range vpaList.Items {
		if vpa.Spec.TargetRef == nil ||
			(vpa.Spec.TargetRef.APIVersion == druidcorev1alpha1.SchemeGroupVersion.String() && vpa.Spec.TargetRef.Kind == "Etcd") {
			continue
		}
		vpaObject := vpa.DeepCopy()

		fns = append(fns, func(ctx context.Context) error {
			original := vpaObject.DeepCopy()

			vpaObject.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
				APIVersion: druidcorev1alpha1.SchemeGroupVersion.String(),
				Kind:       "Etcd",
				Name:       vpaObject.Spec.TargetRef.Name,
			}

			patch := client.MergeFrom(original)
			if err := c.Patch(ctx, vpaObject, patch); err != nil {
				return fmt.Errorf("failed to patch VPA %s: %w", vpaObject.Name, err)
			}
			return nil
		})
	}

	return flow.Parallel(fns...)(ctx)
}
