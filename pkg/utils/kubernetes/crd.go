// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	// WaitTimeout specifies the total time to wait for CRDs to become ready or to be deleted. Exposed for testing.
	// While waiting for CRD readiness is parallelized (see WaitUntilCRDManifestsReady below), the controllers
	// responsible for populating the "readiness" status into the CRD only have one worker each (e.g., see
	// https://github.com/kubernetes/apiextensions-apiserver/blob/376adbc0c7f0bc548dbbf2ad7c4f3e53840aa08f/pkg/controller/establish/establishing_controller.go#L88-L89).
	// Therefore, we need to wait for a longer time here  (basically proportional to the
	// amount of CRDs) in case we create a lot of CRDs in parallel (which happens at Garden or Seed creation), since
	// they are processed sequentially.
	WaitTimeout = 2 * time.Minute
)

// WaitUntilCRDManifestsReady takes names of CRDs and waits for them to get ready with a timeout of 15 seconds.
func WaitUntilCRDManifestsReady(ctx context.Context, c client.Client, crdNames ...string) error {
	var fns []flow.TaskFn
	for _, crdName := range crdNames {
		fns = append(fns, func(ctx context.Context) error {
			timeoutCtx, cancel := context.WithTimeout(ctx, WaitTimeout)
			defer cancel()

			return retry.Until(timeoutCtx, 1*time.Second, func(ctx context.Context) (done bool, err error) {
				crd := &apiextensionsv1.CustomResourceDefinition{}

				if err := c.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
					if client.IgnoreNotFound(err) == nil {
						return retry.MinorError(err)
					}
					return retry.SevereError(err)
				}

				if err := health.CheckCustomResourceDefinition(crd); err != nil {
					return retry.MinorError(err)
				}
				return retry.Ok()
			})
		})
	}
	return flow.Parallel(fns...)(ctx)
}

// WaitUntilCRDManifestsDestroyed takes CRD names and waits for them to be gone with a timeout of 15 seconds.
func WaitUntilCRDManifestsDestroyed(ctx context.Context, c client.Client, crdNames ...string) error {
	var fns []flow.TaskFn

	for _, resourceName := range crdNames {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
			},
		}

		fns = append(fns, func(ctx context.Context) error {
			timeoutCtx, cancel := context.WithTimeout(ctx, WaitTimeout)
			defer cancel()
			return WaitUntilResourceDeleted(timeoutCtx, c, crd, 1*time.Second)
		})
	}
	return flow.Parallel(fns...)(ctx)
}

var (
	crdScheme  *runtime.Scheme
	crdCodec   runtime.Codec
	initOnceFn sync.Once
)

func init() {
	crdScheme = runtime.NewScheme()
	utilruntime.Must(apiextensionsv1.AddToScheme(crdScheme))
	ser := json.NewSerializerWithOptions(json.DefaultMetaFactory, crdScheme, crdScheme, json.SerializerOptions{
		Yaml:   true,
		Pretty: false,
		Strict: false,
	})
	versions := schema.GroupVersions([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
	crdCodec = serializer.NewCodecFactory(crdScheme).CodecForVersions(ser, ser, versions, versions)
}

// DecodeCRD decodes a CRD from a YAML string.
func DecodeCRD(crdYAML string) (*apiextensionsv1.CustomResourceDefinition, error) {
	obj, err := runtime.Decode(crdCodec, []byte(crdYAML))
	if err != nil {
		return nil, fmt.Errorf("failed to decode CRD: %w", err)
	}
	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return nil, fmt.Errorf("expected *apiextensionsv1.CustomResourceDefinition, got %T", obj)
	}
	return crd, nil
}
