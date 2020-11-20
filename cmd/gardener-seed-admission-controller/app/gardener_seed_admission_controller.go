// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/seedadmission"
	"github.com/gardener/gardener/pkg/version/verflag"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var logger = gardenerlogger.NewLogger("info")

// Options has all the context and parameters needed to run a Gardener seed admission controller.
type Options struct {
	// BindAddress is the address the HTTP server should bind to.
	BindAddress string
	// Port is the port that should be opened by the HTTP server.
	Port int
	// ServerCertPath is the path to a server certificate.
	ServerCertPath string
	// ServerKeyPath is the path to a TLS private key.
	ServerKeyPath string
	// Kubeconfig is path to a kubeconfig file. If not given it uses the in-cluster config.
	Kubeconfig string
}

// AddFlags adds flags for a specific Scheduler to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.BindAddress, "bind-address", "0.0.0.0", "address to bind to")
	fs.IntVar(&o.Port, "port", 9443, "server port")
	fs.StringVar(&o.ServerCertPath, "tls-cert-path", "", "path to server certificate")
	fs.StringVar(&o.ServerKeyPath, "tls-private-key-path", "", "path to client certificate")
	fs.StringVar(&o.Kubeconfig, "kubeconfig", "", "path to a kubeconfig")
}

// Validate validates all the required options.
func (o *Options) validate(args []string) error {
	if len(o.BindAddress) == 0 {
		return fmt.Errorf("missing bind address")
	}

	if o.Port == 0 {
		return fmt.Errorf("missing port")
	}

	if len(o.ServerCertPath) == 0 {
		return fmt.Errorf("missing server tls cert path")
	}

	if len(o.ServerKeyPath) == 0 {
		return fmt.Errorf("missing server tls key path")
	}

	if len(args) != 0 {
		return errors.New("arguments are not supported")
	}

	return nil
}

func (o *Options) run(ctx context.Context) {
	run(ctx, o.BindAddress, o.Port, o.ServerCertPath, o.ServerKeyPath, o.Kubeconfig)
}

// NewCommandStartGardenerSeedAdmissionController creates a *cobra.Command object with default parameters
func NewCommandStartGardenerSeedAdmissionController(ctx context.Context) *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "gardener-seed-admission-controller",
		Short: "Launch the Gardener seed admission controller",
		Long:  `The Gardener seed admission controller serves a validation webhook endpoint for resources in the seed clusters.`,
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()

			utilruntime.Must(opts.validate(args))

			logger.Infof("Starting Gardener seed admission controller...")
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				logger.Infof("FLAG: --%s=%s", flag.Name, flag.Value)
			})

			opts.run(ctx)
		},
	}

	flags := cmd.Flags()
	verflag.AddFlags(flags)
	opts.AddFlags(flags)
	return cmd
}

// run runs the Gardener seed admission controller. This should never exit.
func run(ctx context.Context, bindAddress string, port int, certPath, keyPath, kubeconfigPath string) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(admissionregistrationv1beta1.AddToScheme(scheme))
	deserializer := serializer.NewCodecFactory(scheme).UniversalDeserializer()

	mux := http.NewServeMux()

	k8sClient, err := kubernetes.NewClientFromFile("", kubeconfigPath, kubernetes.WithClientOptions(client.Options{
		Scheme: kubernetes.SeedScheme,
	}))
	if err != nil {
		logger.Errorf("unable to create kubernetes client: %+v", err)
		panic(err)
	}

	seedAdmissionController := &GardenerSeedAdmissionController{
		deserializer,
		k8sClient.DirectClient(),
		metav1.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"},
	}

	mux.HandleFunc("/webhooks/validate-extension-crd-deletion", seedAdmissionController.validateExtensionCRDDeletion)
	mux.HandleFunc(
		// in the future we might want to have additional scheduler names
		// so lets have the handler be of pattern "/webhooks/default-pod-scheduler-name/{scheduler-name}"
		fmt.Sprintf(seedadmission.GardenerShootControlPlaneSchedulerWebhookPath),
		seedAdmissionController.defaultShootControlPlanePodsSchedulerName,
	)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", bindAddress, port),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServeTLS(certPath, keyPath); err != http.ErrServerClosed {
			logger.Errorf("Could not start HTTPS server: %v", err)
			panic(err)
		}
	}()

	<-ctx.Done()
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(timeoutCtx); err != nil {
		logger.Errorf("Error when shutting down HTTPS server: %v", err)
	}
	logger.Info("HTTPS servers stopped.")
}

func respond(w http.ResponseWriter, request *admission.Request, response admission.Response) {
	if request != nil {
		if err := response.Complete(*request); err != nil {
			logger.Error(err)
		}
	}

	jsonResponse, err := json.Marshal(admissionv1beta1.AdmissionReview{Response: &response.AdmissionResponse})
	if err != nil {
		logger.Error(err)
	}

	if _, err := w.Write(jsonResponse); err != nil {
		logger.Error(err)
	}
}

// GardenerSeedAdmissionController represents all the parameters required to start the
// Gardener seed admission controller.
type GardenerSeedAdmissionController struct {
	codecs runtime.Decoder
	client client.Client
	podGVK metav1.GroupVersionKind
}

func (g *GardenerSeedAdmissionController) handleAdmissionReview(w http.ResponseWriter, r *http.Request) (admissionv1beta1.AdmissionReview, error) {
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
		respond(w, nil, admission.Errored(http.StatusBadRequest, err))
		return receivedReview, err
	}

	// Deserialize HTTP request body into admissionv1beta1.AdmissionReview object.
	if _, _, err := g.codecs.Decode(body, nil, &receivedReview); err != nil {
		err = fmt.Errorf("invalid request body (error decoding body): %+v", err)
		logger.Errorf(err.Error())
		respond(w, nil, admission.Errored(http.StatusBadRequest, err))
		return receivedReview, err
	}

	// If the request field is empty then do not admit (invalid body).
	if receivedReview.Request == nil {
		err := fmt.Errorf("invalid request body (missing admission request)")
		logger.Errorf(err.Error())
		respond(w, nil, admission.Errored(http.StatusBadRequest, err))
		return receivedReview, err
	}

	return receivedReview, nil
}

func (g *GardenerSeedAdmissionController) validateExtensionCRDDeletion(w http.ResponseWriter, r *http.Request) {
	receivedReview, err := g.handleAdmissionReview(w, r)
	if err != nil {
		return
	}

	// If the request does not indicate the correct operation (DELETE) we allow the review without further doing.
	if receivedReview.Request.Operation != admissionv1beta1.Delete {
		respond(w, nil, admission.Allowed("operation is no DELETE operation"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := seedadmission.ValidateExtensionDeletion(ctx, g.client, logger, receivedReview.Request); err != nil {
		logger.Errorf(err.Error())
		respond(w, nil, admission.Errored(http.StatusBadRequest, err))
		return
	}

	respond(w, nil, admission.Allowed(""))
}

// defaultShootControlPlanePodsSchedulerName sets "gardener-shoot-controlplane-scheduler"
// as schedulerName on Pods.
func (g *GardenerSeedAdmissionController) defaultShootControlPlanePodsSchedulerName(w http.ResponseWriter, r *http.Request) {
	receivedReview, err := g.handleAdmissionReview(w, r)
	if err != nil {
		return
	}

	// If the request does not indicate the correct operation (CREATE) we allow the review without further doing.
	if receivedReview.Request.Operation != admissionv1beta1.Create {
		respond(w, nil, admission.Allowed("operation is no CREATE operation"))
		return
	}

	if receivedReview.Request.Kind != g.podGVK {
		respond(w, nil, admission.Allowed("requested resource is not a pod"))
		return
	}

	if receivedReview.Request.SubResource != "" {
		respond(w, nil, admission.Allowed("subresources on pods are not supported"))
		return
	}

	resp := admission.Allowed("")
	resp.Patches = []jsonpatch.Operation{
		jsonpatch.NewPatch("replace", "/spec/schedulerName", seedadmission.GardenerShootControlPlaneSchedulerName),
	}

	respond(w, &admission.Request{AdmissionRequest: *receivedReview.Request}, resp)
}
