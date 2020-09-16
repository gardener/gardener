// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"

	coreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	"github.com/gardener/gardener/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

// CreateRecorder creates a record.EventRecorder that is not limited to a namespace having a specific eventSourceName
func CreateRecorder(kubeClient k8s.Interface, eventSourceName string) record.EventRecorder {
	scheme := scheme.Scheme

	coreinstall.Install(scheme)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logger.Logger.Debugf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: typedcorev1.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: eventSourceName})
}

// ContextFromStopChannel creates a new context from a given stop channel.
func ContextFromStopChannel(stopCh <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-stopCh
	}()

	return ctx
}
