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

// IMPORTANT: This is only a temporary component that will be removed as soon as https://github.com/kubernetes/kubernetes/issues/68996 is fixed.
// It mutates PersistentVolumes and removes the initializer for all non-cloud volumes.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const pvLabelInitializerName = "pvlabel.kubernetes.io"

var (
	logger = gardenerlogger.NewLogger("info")
)

func main() {
	stopCh := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-c
		close(stopCh)
		<-c
		os.Exit(1)
	}()

	var (
		bindAddress = flag.String("bind-address", "0.0.0.0", "address to bind to")
		port        = flag.Int("secure-port", 2730, "serverport")
		serverCert  = flag.String("tls-cert-file", "server.crt", "path to server certificate")
		serverKey   = flag.String("tls-private-key-file", "server.key", "path to client certificate")
	)

	flag.Parse()

	serve(*bindAddress, *port, *serverCert, *serverKey, stopCh)
}

func serve(bindAddress string, port int, certPath, keyPath string, stopCh <-chan struct{}) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	admissionregistrationv1beta1.AddToScheme(scheme)
	deserializer := serializer.NewCodecFactory(scheme).UniversalDeserializer()

	mux := http.NewServeMux()

	pvInitializerHandler := &pvInitializerHandler{deserializer}
	mux.HandleFunc("/webhooks/mutate-pv-initializers", pvInitializerHandler.mutate)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", bindAddress, port),
		Handler: mux,
	}

	go func() {
		logger.Infof("Starting Gardener external admission controller...")
		if err := srv.ListenAndServeTLS(certPath, keyPath); err != http.ErrServerClosed {
			logger.Errorf("Could not start HTTPS server: %v", err)
		}
	}()

	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("Error when shutting down HTTPS server: %v", err)
	}
	logger.Info("HTTPS servers stopped.")
}

type pvInitializerHandler struct {
	codecs runtime.Decoder
}

func (p *pvInitializerHandler) mutate(w http.ResponseWriter, r *http.Request) {
	var (
		body           []byte
		receivedReview = admissionv1beta1.AdmissionReview{}
	)

	// Read HTTP request body into variable.
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// Verify that the correct content-type header has been sent.
	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		err := fmt.Errorf("contentType=%s, expect %s", contentType, "application/json")
		logger.Errorf(err.Error())
		respond(w, &admissionv1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}})
		return
	}

	// Deserialize HTTP request body into admissionv1beta1.AdmissionReview object.
	if _, _, err := p.codecs.Decode(body, nil, &receivedReview); err != nil {
		logger.Errorf(err.Error())
		respond(w, &admissionv1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}})
		return
	}

	// If the request field is empty then do not admit (invalid body).
	if receivedReview.Request == nil {
		err := fmt.Errorf("invalid request body (missing admission request)")
		logger.Errorf(err.Error())
		respond(w, &admissionv1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}})
		return
	}
	// If the request does not indicate the correct operation (CREATE,UPDATE) we allow the review without further doing.
	if receivedReview.Request.Operation != admissionv1beta1.Create && receivedReview.Request.Operation != admissionv1beta1.Update {
		respond(w, &admissionv1beta1.AdmissionResponse{})
		return
	}

	// Check that resource is a persistentvolume
	persistentVolumeResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"}
	if receivedReview.Request.Resource != persistentVolumeResource {
		err := fmt.Errorf("invalid object (no persistentvolume)")
		logger.Errorf(err.Error())
		respond(w, &admissionv1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}})
		return
	}

	// At this point all validation steps have been passed, let's mutate our object.
	pv := &corev1.PersistentVolume{}
	if err := json.Unmarshal(receivedReview.Request.Object.Raw, pv); err != nil {
		logger.Errorf(err.Error())
		respond(w, &admissionv1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}})
		return
	}

	patch := "[]"
	if pv.DeletionTimestamp == nil && isCloudSpecificPersistentVolume(pv) {
		switch {
		case pv.Initializers == nil:
			patch = fmt.Sprintf(`[{"op": "add", "path": "/metadata/initializers", "value": {"pending": [{"name": "%s"}]}}]`, pvLabelInitializerName)
		case !common.HasInitializer(pv.Initializers, pvLabelInitializerName):
			patch = fmt.Sprintf(`[{"op": "add", "path": "/metadata/initializers/pending/%d", "value": {"name": "%s"}}]`, len(pv.Initializers.Pending), pvLabelInitializerName)
		}
		logger.Infof("Added initializer '%s' for PersistentVolume '%s'", pvLabelInitializerName, pv.Name)
	} else {
		logger.Infof("Skipped PersistentVolume '%s' as it is marked for deletion or not cloud specific", pv.Name)
	}

	respond(w, &admissionv1beta1.AdmissionResponse{
		UID:     receivedReview.Request.UID,
		Allowed: true,
		Patch:   []byte(patch),
		PatchType: func() *admissionv1beta1.PatchType {
			pt := admissionv1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	})
}

func respond(w http.ResponseWriter, response *admissionv1beta1.AdmissionResponse) {
	responseObj := admissionv1beta1.AdmissionReview{}
	if response != nil {
		responseObj.Response = response
	}

	jsonResponse, err := json.Marshal(responseObj)
	if err != nil {
		logger.Error(err)
	}
	if _, err := w.Write(jsonResponse); err != nil {
		logger.Error(err)
	}
}

func isCloudSpecificPersistentVolume(pv *corev1.PersistentVolume) bool {
	volumeSource := pv.Spec.PersistentVolumeSource
	return volumeSource.AWSElasticBlockStore != nil ||
		volumeSource.GCEPersistentDisk != nil ||
		volumeSource.AzureDisk != nil ||
		volumeSource.AzureFile != nil ||
		volumeSource.VsphereVolume != nil ||
		volumeSource.PhotonPersistentDisk != nil
}
