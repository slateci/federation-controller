/*
Copyright 2022 SLATE project.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha2

import (
	"context"
	time "time"

	federationcontrollerv1alpha2 "github.com/slateci/federation-controller/pkg/apis/federationcontroller/v1alpha2"
	versioned "github.com/slateci/federation-controller/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/slateci/federation-controller/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha2 "github.com/slateci/federation-controller/pkg/generated/listers/federationcontroller/v1alpha2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ClusterNSInformer provides access to a shared informer and lister for
// ClusterNSs.
type ClusterNSInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha2.ClusterNSLister
}

type clusterNSInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewClusterNSInformer constructs a new informer for ClusterNS type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewClusterNSInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredClusterNSInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredClusterNSInformer constructs a new informer for ClusterNS type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredClusterNSInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.FederationcontrollerV1alpha2().ClusterNSs(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.FederationcontrollerV1alpha2().ClusterNSs(namespace).Watch(context.TODO(), options)
			},
		},
		&federationcontrollerv1alpha2.ClusterNS{},
		resyncPeriod,
		indexers,
	)
}

func (f *clusterNSInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredClusterNSInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *clusterNSInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&federationcontrollerv1alpha2.ClusterNS{}, f.defaultInformer)
}

func (f *clusterNSInformer) Lister() v1alpha2.ClusterNSLister {
	return v1alpha2.NewClusterNSLister(f.Informer().GetIndexer())
}
