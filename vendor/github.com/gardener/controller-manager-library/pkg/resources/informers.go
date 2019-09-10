/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/gardener/controller-manager-library/pkg/logger"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type GenericInformer interface {
	cache.SharedIndexInformer
	Informer() cache.SharedIndexInformer
	Lister() Lister
}

type genericInformer struct {
	cache.SharedIndexInformer
	resource *Info
}

func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.SharedIndexInformer
}

func (f *genericInformer) Lister() Lister {
	return NewLister(f.Informer().GetIndexer(), f.resource)
}

// SharedInformerFactory provides shared informers for resources in all known
// API group versions.
type SharedInformerFactory interface {
	Structured() GenericFilteredInformerFactory
	Unstructured() GenericFilteredInformerFactory

	InformerForObject(obj runtime.Object) (GenericInformer, error)
	FilteredInformerForObject(obj runtime.Object, namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error)

	InformerFor(gvk schema.GroupVersionKind) (GenericInformer, error)
	FilteredInformerFor(gvk schema.GroupVersionKind, namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error)

	UnstructuredInformerFor(gvk schema.GroupVersionKind) (GenericInformer, error)
	FilteredUnstructuredInformerFor(gvk schema.GroupVersionKind, namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error)

	Start(stopCh <-chan struct{})
	WaitForCacheSync(stopCh <-chan struct{})
}

type GenericInformerFactory interface {
	InformerFor(gvk schema.GroupVersionKind) (GenericInformer, error)
	Start(stopCh <-chan struct{})
	WaitForCacheSync(stopCh <-chan struct{})
}

type GenericFilteredInformerFactory interface {
	GenericInformerFactory
	FilteredInformerFor(gvk schema.GroupVersionKind, namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error)
	LookupInformerFor(gvk schema.GroupVersionKind, namespace string) (GenericInformer, error)
}

///////////////////////////////////////////////////////////////////////////////
//  informer factory

type sharedInformerFactory struct {
	context      *resourceContext
	structured   *sharedFilteredInformerFactory
	unstructured *unstructuredSharedFilteredInformerFactory
}

func newSharedInformerFactory(rctx *resourceContext, defaultResync time.Duration) *sharedInformerFactory {
	return &sharedInformerFactory{
		context:      rctx,
		structured:   newSharedFilteredInformerFactory(rctx, defaultResync),
		unstructured: &unstructuredSharedFilteredInformerFactory{newSharedFilteredInformerFactory(rctx, defaultResync)},
	}
}

func (f *sharedInformerFactory) Structured() GenericFilteredInformerFactory {
	return f
}

func (f *sharedInformerFactory) Unstructured() GenericFilteredInformerFactory {
	return f.unstructured
}

// Start initializes all requested informers.
func (f *sharedInformerFactory) Start(stopCh <-chan struct{}) {
	f.structured.Start(stopCh)
	f.unstructured.Start(stopCh)
}

func (f *sharedInformerFactory) WaitForCacheSync(stopCh <-chan struct{}) {
	f.structured.WaitForCacheSync(stopCh)
	f.unstructured.WaitForCacheSync(stopCh)
}

func (f *sharedInformerFactory) UnstructuredInformerFor(gvk schema.GroupVersionKind) (GenericInformer, error) {
	return f.unstructured.informerFor(unstructuredType, gvk, "", nil)
}

func (f *sharedInformerFactory) FilteredUnstructuredInformerFor(gvk schema.GroupVersionKind, namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error) {
	return f.unstructured.informerFor(unstructuredType, gvk, namespace, optionsFunc)
}

func (f *sharedInformerFactory) InformerFor(gvk schema.GroupVersionKind) (GenericInformer, error) {
	return f.FilteredInformerFor(gvk, "", nil)
}

func (f *sharedInformerFactory) FilteredInformerFor(gvk schema.GroupVersionKind, namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error) {
	informerType := f.context.KnownTypes(gvk.GroupVersion())[gvk.Kind]
	if informerType == nil {
		return nil, fmt.Errorf("%s unknown", gvk)
	}

	return f.structured.informerFor(informerType, gvk, namespace, optionsFunc)
}

func (f *sharedInformerFactory) LookupInformerFor(gvk schema.GroupVersionKind, namespace string) (GenericInformer, error) {
	informerType := f.context.KnownTypes(gvk.GroupVersion())[gvk.Kind]
	if informerType == nil {
		return nil, fmt.Errorf("%s unknown", gvk)
	}

	return f.structured.lookupInformerFor(informerType, gvk, namespace)
}

func (f *sharedInformerFactory) InformerForObject(obj runtime.Object) (GenericInformer, error) {
	return f.FilteredInformerForObject(obj, "", nil)
}

func (f *sharedInformerFactory) FilteredInformerForObject(obj runtime.Object, namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error) {
	informerType := reflect.TypeOf(obj)
	for informerType.Kind() == reflect.Ptr {
		informerType = informerType.Elem()
	}

	gvk, err := f.context.GetGVK(obj)
	if err != nil {
		return nil, err
	}
	return f.structured.informerFor(informerType, gvk, namespace, optionsFunc)
}

///////////////////////////////////////////////////////////////////////////////
// Shared Filtered Informer Factory

type sharedFilteredInformerFactory struct {
	lock sync.Mutex

	context       *resourceContext
	defaultResync time.Duration
	filters       map[string]*genericInformerFactory
}

func newSharedFilteredInformerFactory(rctx *resourceContext, defaultResync time.Duration) *sharedFilteredInformerFactory {
	return &sharedFilteredInformerFactory{
		context:       rctx,
		defaultResync: defaultResync,

		filters: make(map[string]*genericInformerFactory),
	}
}

// Start initializes all requested informers.
func (f *sharedFilteredInformerFactory) Start(stopCh <-chan struct{}) {
	for _, i := range f.filters {
		i.Start(stopCh)
	}
}

func (f *sharedFilteredInformerFactory) WaitForCacheSync(stopCh <-chan struct{}) {
	for _, i := range f.filters {
		i.WaitForCacheSync(stopCh)
	}
}

func (f *sharedFilteredInformerFactory) getFactory(namespace string, optionsFunc TweakListOptionsFunc) *genericInformerFactory {
	key := namespace
	if optionsFunc != nil {
		opts := v1.ListOptions{}
		optionsFunc(&opts)
		key = namespace + opts.String()
	}

	f.lock.Lock()
	defer f.lock.Unlock()

	factory, exists := f.filters[key]
	if !exists {
		factory = newGenericInformerFactory(f.context, f.defaultResync, namespace, optionsFunc)
		f.filters[key] = factory
	}
	return factory
}

func (f *sharedFilteredInformerFactory) queryFactory(namespace string) *genericInformerFactory {
	f.lock.Lock()
	defer f.lock.Unlock()

	factory, _ := f.filters[namespace]
	return factory
}

func (f *sharedFilteredInformerFactory) informerFor(informerType reflect.Type, gvk schema.GroupVersionKind, namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error) {
	return f.getFactory(namespace, optionsFunc).informerFor(informerType, gvk)
}

func (f *sharedFilteredInformerFactory) lookupInformerFor(informerType reflect.Type, gvk schema.GroupVersionKind, namespace string) (GenericInformer, error) {
	fac := f.queryFactory("")
	if fac != nil {
		i := fac.queryInformerFor(informerType, gvk)
		if i != nil {
			return i, nil
		}
	}
	if namespace != "" {
		fac := f.queryFactory(namespace)
		if fac != nil {
			i := fac.queryInformerFor(informerType, gvk)
			if i != nil {
				return i, nil
			}
			return fac.informerFor(informerType, gvk)
		}
	}
	return f.getFactory("", nil).informerFor(informerType, gvk)
}

///////////////////////////////////////////////////////////////////////////////
// UnstructuredInformers

type unstructuredSharedFilteredInformerFactory struct {
	*sharedFilteredInformerFactory
}

func (f *unstructuredSharedFilteredInformerFactory) InformerFor(gvk schema.GroupVersionKind) (GenericInformer, error) {
	return f.informerFor(unstructuredType, gvk, "", nil)
}

func (f *unstructuredSharedFilteredInformerFactory) FilteredInformerFor(gvk schema.GroupVersionKind, namespace string, optionsFunc TweakListOptionsFunc) (GenericInformer, error) {
	return f.informerFor(unstructuredType, gvk, namespace, optionsFunc)
}

func (f *unstructuredSharedFilteredInformerFactory) LookupInformerFor(gvk schema.GroupVersionKind, namespace string) (GenericInformer, error) {
	return f.lookupInformerFor(unstructuredType, gvk, namespace)
}

////////////////////////////////////////////////////////////////////////////////
// Watch

type watchWrapper struct {
	ctx        context.Context
	orig       watch.Interface
	origChan   <-chan watch.Event
	resultChan chan watch.Event
}

func NewWatchWrapper(ctx context.Context, orig watch.Interface) watch.Interface {
	logger.Infof("*************** new wrapper ********************")
	w := &watchWrapper{ctx, orig, orig.ResultChan(), make(chan watch.Event)}
	go w.Run()
	return w
}

func (w *watchWrapper) Stop() {
	w.orig.Stop()
}

func (w *watchWrapper) ResultChan() <-chan watch.Event {
	return w.resultChan
}
func (w *watchWrapper) Run() {
loop:
	for {
		select {
		case <-w.ctx.Done():
			break loop
		case e, ok := <-w.origChan:
			if !ok {
				logger.Infof("watch aborted")
				break loop
			} else {
				logger.Infof("WATCH: %#v\n", e)
				w.resultChan <- e
			}
		}
	}
	logger.Infof("stop wrapper ***************")
	close(w.resultChan)
}

var _ watch.Interface = &watchWrapper{}
