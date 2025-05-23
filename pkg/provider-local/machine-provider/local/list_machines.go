// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"fmt"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1alpha1 "github.com/gardener/gardener/pkg/provider-local/machine-provider/api/v1alpha1"
)

func (d *localDriver) ListMachines(ctx context.Context, req *driver.ListMachinesRequest) (*driver.ListMachinesResponse, error) {
	if req.MachineClass.Provider != apiv1alpha1.Provider {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("requested for Provider '%s', we only support '%s'", req.MachineClass.Provider, apiv1alpha1.Provider))
	}

	klog.V(3).Infof("Machine list request has been received for %q", req.MachineClass.Name)
	defer klog.V(3).Infof("Machine list request has been processed for %q", req.MachineClass.Name)

	podList := &corev1.PodList{}
	if err := d.client.List(ctx, podList, client.InNamespace(req.MachineClass.Namespace), client.MatchingLabels{labelKeyProvider: apiv1alpha1.Provider}); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	machineList := make(map[string]string, len(podList.Items))
	for _, pod := range podList.Items {
		machineList[pod.Name] = machineName(pod.Name)
	}

	return &driver.ListMachinesResponse{MachineList: machineList}, nil
}
