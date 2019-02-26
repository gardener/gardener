// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
)

type metrics struct {
	k8sGardenClient kubernetes.Interface
	interval        time.Duration
}

// InitMetrics takes an Kubernetes <client> for a Garden cluster and initiate the
// collection of Gardener and Shoot related metrics. It returns a <http.Handler> to
// register on a webserver, which provides the collected metrics in the Prometheus format.
func InitMetrics(client kubernetes.Interface, scrapeInterval time.Duration) http.Handler {
	m := metrics{
		k8sGardenClient: client,
		interval:        scrapeInterval,
	}
	m.initShootMetrics()
	m.initProjectCountMetric()
	m.initUserCountMetric()
	return promhttp.Handler()
}

func (m metrics) initShootMetrics() {
	metricShootState := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_shoot_state",
		Help: "State of a Shoot cluster",
	}, []string{"name", "project", "cloud", "version", "region", "seed", "operation", "is_seed", "mail_to"})

	metricShootNodeCount := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_shoot_nodes_count",
		Help: "Node count of a Shoot cluster",
	}, []string{"name", "project"})

	metricShootStateConditions := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "garden_shoot_state_conditions",
		Help: "Conditions of a Shoot cluster",
	}, []string{"name", "project", "condition", "status", "operation", "mail_to"})

	prometheus.Register(metricShootState)
	prometheus.Register(metricShootNodeCount)
	prometheus.Register(metricShootStateConditions)

	m.collect(func() {
		shoots, err := m.k8sGardenClient.Garden().GardenV1beta1().Shoots(metav1.NamespaceAll).List(metav1.ListOptions{})
		if err != nil {
			logger.Logger.Info("Unable to fetch shoots. skip shoot metric set...")
			return
		}

		for _, shoot := range shoots.Items {
			var (
				mailTo         string
				nodeCount      int
				operationState float64 = 4 // unknown
				operationType          = "Unknown"
				isSeed                 = "False"
			)

			if shoot.Annotations != nil {
				mailTo = shoot.Annotations[common.GardenOperatedBy]

				ok, err := strconv.ParseBool(shoot.Annotations[common.ShootUseAsSeed])
				if err == nil && ok {
					isSeed = "True"
				}
			}

			if shoot.Status.LastOperation != nil {
				operationType = string(shoot.Status.LastOperation.Type)

				switch shoot.Status.LastOperation.State {
				case gardencorev1alpha1.LastOperationStateSucceeded:
					operationState = 0
				case gardencorev1alpha1.LastOperationStateProcessing:
					operationState = 1
				case gardencorev1alpha1.LastOperationStateError:
					operationState = 2
				case gardencorev1alpha1.LastOperationStateFailed:
					operationState = 3
				}
			}

			cloud, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
			if err != nil {
				logger.Logger.Infof("Unable to determine cloud provider for Shoot %s", shoot.Name)
				return
			}

			metricShootState.With(prometheus.Labels{
				"name":      shoot.Name,
				"project":   shoot.Namespace,
				"cloud":     string(cloud),
				"region":    shoot.Spec.Cloud.Region,
				"version":   shoot.Spec.Kubernetes.Version,
				"seed":      *(shoot.Spec.Cloud.Seed),
				"operation": operationType,
				"is_seed":   isSeed,
				"mail_to":   mailTo,
			}).Set(operationState)

			for _, condition := range shoot.Status.Conditions {
				var conditionStatus float64
				if condition.Status == gardencorev1alpha1.ConditionTrue {
					conditionStatus = 1
				}
				metricShootStateConditions.With(prometheus.Labels{
					"name":      shoot.Name,
					"project":   shoot.Namespace,
					"condition": string(condition.Type),
					"status":    string(condition.Status),
					"operation": operationType,
					"mail_to":   mailTo,
				}).Set(conditionStatus)
			}

			// Collect the count of nodes
			switch cloud {
			case gardenv1beta1.CloudProviderAWS:
				for _, worker := range shoot.Spec.Cloud.AWS.Workers {
					nodeCount += worker.AutoScalerMax
				}
			case gardenv1beta1.CloudProviderAzure:
				for _, worker := range shoot.Spec.Cloud.Azure.Workers {
					nodeCount += worker.AutoScalerMax
				}
			case gardenv1beta1.CloudProviderGCP:
				for _, worker := range shoot.Spec.Cloud.GCP.Workers {
					nodeCount += worker.AutoScalerMax
				}
			case gardenv1beta1.CloudProviderOpenStack:
				for _, worker := range shoot.Spec.Cloud.OpenStack.Workers {
					nodeCount += worker.AutoScalerMax
				}
			case gardenv1beta1.CloudProviderLocal:
				nodeCount = 1
			}
			metricShootNodeCount.With(prometheus.Labels{
				"name":    shoot.Name,
				"project": shoot.Namespace,
			}).Set(float64(nodeCount))
		}
	})
}

func (m metrics) initProjectCountMetric() {
	metricProjectCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "garden_project_count",
		Help: "Count of projects",
	})
	prometheus.MustRegister(metricProjectCount)
	projectsLabel := fmt.Sprintf("%s=%s", common.GardenRole, common.GardenRoleProject)

	m.collect(func() {
		projects, err := m.k8sGardenClient.ListNamespaces(metav1.ListOptions{
			LabelSelector: projectsLabel,
		})
		if err != nil {
			logger.Logger.Info("Unable to fetch project namespaces. skip metric...")
			return
		}
		metricProjectCount.Set(float64(len(projects.Items)))
	})
}

func (m metrics) initUserCountMetric() {
	metricUserCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "garden_user_count",
		Help: "Count of users",
	})
	prometheus.MustRegister(metricUserCount)
	usersLabel := fmt.Sprintf("%s=%s", common.GardenRole, common.GardenRoleMembers)

	m.collect(func() {
		roleBindings, err := m.k8sGardenClient.ListRoleBindings(metav1.NamespaceAll, metav1.ListOptions{
			LabelSelector: usersLabel,
		})
		if err != nil {
			logger.Logger.Info("Unable to fetch user RoleBindings. skip metric...")
			return
		}
		users := sets.NewString()
		for _, rb := range roleBindings.Items {
			for _, subject := range rb.Subjects {
				if subject.Kind == "User" && !users.Has(subject.Name) {
					users.Insert(subject.Name)
				}
			}
		}
		metricUserCount.Set(float64(users.Len()))
	})
}

func (m metrics) collect(metricsCollector func()) {
	go func() {
		for {
			metricsCollector()
			time.Sleep(m.interval)
		}
	}()
}
