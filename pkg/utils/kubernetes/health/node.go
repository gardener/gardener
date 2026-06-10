// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getNodeCondition(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

var (
	trueNodeConditionTypes = []corev1.NodeConditionType{
		corev1.NodeReady,
	}

	falseNodeConditionTypes = []corev1.NodeConditionType{
		corev1.NodeDiskPressure,
		corev1.NodeMemoryPressure,
		corev1.NodeNetworkUnavailable,
		corev1.NodePIDPressure,
	}
)

// CheckNode checks whether the given Node is healthy.
// A node is considered healthy if it has a `corev1.NodeReady` condition and this condition reports
// `corev1.ConditionTrue`.
func CheckNode(node *corev1.Node) error {
	for _, trueConditionType := range trueNodeConditionTypes {
		conditionType := string(trueConditionType)
		condition := getNodeCondition(node.Status.Conditions, trueConditionType)
		if condition == nil {
			return requiredConditionMissing(conditionType)
		}
		if err := checkConditionState(string(condition.Type), string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	for _, falseConditionType := range falseNodeConditionTypes {
		condition := getNodeCondition(node.Status.Conditions, falseConditionType)
		if condition == nil {
			continue
		}
		if err := checkConditionState(string(condition.Type), string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
			return err
		}
	}

	return nil
}

// GetPreservedNodeNames returns the names of nodes that belong to a preserved failed machine —
// i.e. nodes with NodePreserved == True and NodeReady == False or Unknown.
func GetPreservedNodeNames(ctx context.Context, c client.Client) (sets.Set[string], error) {
	nodeList := &corev1.NodeList{}
	if err := c.List(ctx, nodeList); err != nil {
		return nil, err
	}
	preserved := sets.New[string]()
	for _, node := range nodeList.Items {
		if IsNodePreservedFailed(node) {
			preserved.Insert(node.Name)
		}
	}
	return preserved, nil
}

// IsNodePreservedFailed reports whether a node belongs to a preserved failed machine: it must have
// NodePreserved == True and NodeReady == False or Unknown.
func IsNodePreservedFailed(node corev1.Node) bool {
	isPreserved := false
	isNotReady := false
	for _, condition := range node.Status.Conditions {
		if condition.Type == machinev1alpha1.NodePreserved && condition.Status == corev1.ConditionTrue {
			isPreserved = true
		} else if condition.Type == corev1.NodeReady && (condition.Status == corev1.ConditionFalse || condition.Status == corev1.ConditionUnknown) {
			isNotReady = true
		}
		if isPreserved && isNotReady {
			return true
		}
	}
	return false
}
