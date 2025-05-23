// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReadLeaderElectionRecord returns the leader election record for a given lock type and a namespace/name combination.
func ReadLeaderElectionRecord(ctx context.Context, c client.Client, lock, namespace, name string) (*resourcelock.LeaderElectionRecord, error) {
	switch lock {
	case "endpoints":
		endpoint := &corev1.Endpoints{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, endpoint); err != nil {
			return nil, err
		}
		return leaderElectionRecordFromAnnotations(endpoint.Annotations)

	case "configmaps":
		configmap := &corev1.ConfigMap{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, configmap); err != nil {
			return nil, err
		}
		return leaderElectionRecordFromAnnotations(configmap.Annotations)

	case resourcelock.LeasesResourceLock:
		lease := &coordinationv1.Lease{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, lease); err != nil {
			return nil, err
		}
		return resourcelock.LeaseSpecToLeaderElectionRecord(&lease.Spec), nil
	}

	return nil, fmt.Errorf("unknown lock type: %s", lock)
}

func leaderElectionRecordFromAnnotations(annotations map[string]string) (*resourcelock.LeaderElectionRecord, error) {
	var leaderElectionRecord resourcelock.LeaderElectionRecord

	leaderElection, ok := annotations[resourcelock.LeaderElectionRecordAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("could not find key %q in annotations", resourcelock.LeaderElectionRecordAnnotationKey)
	}

	if err := json.Unmarshal([]byte(leaderElection), &leaderElectionRecord); err != nil {
		return nil, fmt.Errorf("failed to unmarshal leader election record: %+v", err)
	}

	return &leaderElectionRecord, nil
}
