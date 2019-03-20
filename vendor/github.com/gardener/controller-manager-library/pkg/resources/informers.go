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
	"github.com/gardener/controller-manager-library/pkg/kutil"
	"github.com/gardener/controller-manager-library/pkg/logger"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"reflect"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
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
	InformerForObject(obj runtime.Object) (GenericInformer, error)
	InformerFor(gvk schema.GroupVersionKind) (GenericInformer, error)
	Start(stopCh <-chan struct{})
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool
}

type sharedInformerFactory struct {
	lock sync.Mutex

	context *resourceContext

	defaultResync time.Duration
	informers     map[reflect.Type]GenericInformer
	// startedInformers is used for tracking which informers have been started.
	// This allows Start() to be called multiple times safely.
	startedInformers map[reflect.Type]bool
}

// NewSharedInformerFactory constructs a new instance of sharedInformerFactory for all namespaces.
func (c *resourceContext) SharedInformerFactory() SharedInformerFactory {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.sharedInformerFactory == nil {
		c.sharedInformerFactory = newSharedInformerFactory(c, c.defaultResync)
	}
	return c.sharedInformerFactory
}

func newSharedInformerFactory(rctx *resourceContext, defaultResync time.Duration) *sharedInformerFactory {
	return &sharedInformerFactory{
		context:       rctx,
		defaultResync: defaultResync,

		informers:        make(map[reflect.Type]GenericInformer),
		startedInformers: make(map[reflect.Type]bool),
	}
}

// Start initializes all requested informers.
func (f *sharedInformerFactory) Start(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for informerType, informer := range f.informers {
		if !f.startedInformers[informerType] {
			go informer.Run(stopCh)
			f.startedInformers[informerType] = true
		}
	}
}

// WaitForCacheSync waits for all started informers' cache were synced.
func (f *sharedInformerFactory) WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool {
	informers := func() map[reflect.Type]cache.SharedIndexInformer {
		f.lock.Lock()
		defer f.lock.Unlock()

		informers := map[reflect.Type]cache.SharedIndexInformer{}
		for informerType, informer := range f.informers {
			if f.startedInformers[informerType] {
				informers[informerType] = informer
			}
		}
		return informers
	}()

	res := map[reflect.Type]bool{}
	for informType, informer := range informers {
		res[informType] = cache.WaitForCacheSync(stopCh, informer.HasSynced)
	}
	return res
}
func (f *sharedInformerFactory) InformerFor(gvk schema.GroupVersionKind) (GenericInformer, error) {
	informerType := f.context.KnownTypes(gvk.GroupVersion())[gvk.Kind]
	if informerType == nil {
		return nil, fmt.Errorf("%s unknown", gvk)
	}
	informer, exists := f.informers[informerType]
	if exists {
		return informer, nil
	}
	return f.informerFor(informerType, gvk)
}

func (f *sharedInformerFactory) InformerForObject(obj runtime.Object) (GenericInformer, error) {
	informerType := reflect.TypeOf(obj)
	for informerType.Kind() == reflect.Ptr {
		informerType = informerType.Elem()
	}

	informer, exists := f.informers[informerType]
	if exists {
		return informer, nil
	}

	gvk, err := f.context.GetGVK(obj)
	if err != nil {
		return nil, err
	}
	return f.informerFor(informerType, gvk)
}

func (f *sharedInformerFactory) informerFor(informerType reflect.Type, gvk schema.GroupVersionKind) (GenericInformer, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	informer, exists := f.informers[informerType]
	if exists {
		return informer, nil
	}

	l := kutil.DetermineListType(f.context.Scheme, gvk.GroupVersion(), informerType)
	if l == nil {
		return nil, fmt.Errorf("no list type found for %s", informerType)
	}

	client, err := f.getClient(gvk.GroupVersion())
	if err != nil {
		return nil, err
	}

	info, err := f.context.Get(gvk)
	if err != nil {
		return nil, err
	}
	informer = f.newInformer(client, info, informerType, l)
	f.informers[informerType] = informer

	return informer, nil
}

func (f *sharedInformerFactory) getClient(gv schema.GroupVersion) (restclient.Interface, error) {
	return f.context.GetClient(gv)
}

func (f *sharedInformerFactory) newInformer(client restclient.Interface, res *Info, elemType reflect.Type, listType reflect.Type) GenericInformer {
	logger.Infof("new generic informer for %s (%s) %s (%d seconds)", elemType, res.GroupVersionKind(), listType, f.defaultResync/time.Second)
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				result := reflect.New(listType).Interface().(runtime.Object)
				r := client.Get().
					Resource(res.Name()).
					VersionedParams(&options, f.context.Clients.parametercodec)
				if res.Namespaced() {
					r = r.Namespace("")
				}

				return result, r.Do().Into(result)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.Watch = true
				r := client.Get().
					Resource(res.Name()).
					VersionedParams(&options, f.context.Clients.parametercodec)
				if res.Namespaced() {
					r = r.Namespace("")
				}

				return r.Watch()
			},
		},
		reflect.New(elemType).Interface().(runtime.Object),
		f.defaultResync,
		indexers,
	)
	return &genericInformer{informer, res}
}

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
