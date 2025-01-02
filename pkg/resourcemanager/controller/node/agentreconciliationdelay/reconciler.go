// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package agentreconciliationdelay

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler manages the node-agent.gardener.cloud/reconciliation-delay annotation on nodes.
type Reconciler struct {
	TargetClient client.Client
	Config       resourcemanagerconfigv1alpha1.NodeAgentReconciliationDelayControllerConfig

	knownNodeNames sets.Set[string]
}

// Reconcile computes a time.Duration that can be used to delay reconciliations by using a simple linear mapping
// approach based on the indices of the nodes in the list of all nodes in the cluster. This way, the delays of all
// instances of gardener-node-agent are distributed evenly.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	nodeList := &corev1.NodeList{}
	if err := r.TargetClient.List(ctx, nodeList); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed listing nodes from store: %w", err)
	}

	// nothing to be done, node list has not changed since last time
	currentNodeNames := nodeNames(nodeList)
	if currentNodeNames.Equal(r.knownNodeNames) {
		return reconcile.Result{}, nil
	}

	kubernetesutils.ByName().Sort(nodeList)

	var taskFns []flow.TaskFn

	for index, n := range nodeList.Items {
		node := n

		taskFns = append(taskFns, func(ctx context.Context) error {
			rangeSize := (r.Config.MaxDelay.Seconds() - r.Config.MinDelay.Seconds()) / float64(len(nodeList.Items))
			delaySeconds := r.Config.MinDelay.Seconds() + float64(index)*rangeSize

			log.V(1).Info("Computed reconciliation delay", "delaySeconds", delaySeconds, "nodeName", node.Name)

			patch := client.MergeFrom(node.DeepCopy())
			metav1.SetMetaDataAnnotation(&node.ObjectMeta, v1beta1constants.AnnotationNodeAgentReconciliationDelay, time.Duration(delaySeconds*float64(time.Second)).String())
			return r.TargetClient.Patch(ctx, &node, patch)
		})
	}

	if err := flow.Parallel(taskFns...)(ctx); err != nil {
		return reconcile.Result{}, err
	}

	r.knownNodeNames = currentNodeNames
	return reconcile.Result{}, nil
}

func nodeNames(nodeList *corev1.NodeList) sets.Set[string] {
	out := sets.New[string]()
	for _, node := range nodeList.Items {
		out.Insert(node.Name)
	}
	return out
}
