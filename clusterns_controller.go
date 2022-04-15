/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	"context"
	"fmt"
	rbac "k8s.io/api/rbac/v1"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clientset "github.com/slateci/nrp-clone/pkg/generated/clientset/versioned"
	nrpscheme "github.com/slateci/nrp-clone/pkg/generated/clientset/versioned/scheme"
	informers "github.com/slateci/nrp-clone/pkg/generated/informers/externalversions/nrpcontroller/v1alpha1"
	listers "github.com/slateci/nrp-clone/pkg/generated/listers/nrpcontroller/v1alpha1"
)

const (
	// ClusterNSSuccessSynced is used as part of the Event 'reason' when a Cluster is synced
	ClusterNSSuccessSynced = "Synced"

	// ClusterNSSuccessDeleted is used as part of the Event 'reason' when a Cluster is synced
	ClusterNSSuccessDeleted = "Deleted"

	// ClusterNSErrResourceExists is used as part of the Event 'reason' when a Cluster fails
	// to sync due to a Deployment of the same name already existing.
	ClusterNSErrResourceExists = "ErrResourceExists"

	// ClusterNSMessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	ClusterNSMessageResourceExists = "Resource %q already exists and is not managed by Cluster"
	// ClusterNSMessageResourceSynced is the message used for an Event fired when a Cluster
	// is synced successfully
	ClusterNSMessageResourceSynced = "Cluster synced successfully"
	// ClusterNSMessageResourceDeleted is the message used for an Event fired when a Cluster
	// is deleted successfully
	ClusterNSMessageResourceDeleted = "Cluster deleted successfully"
)

// ClusterNSController is the controller implementation for Cluster resources
type ClusterNSController struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// nrpclientset is a clientset for our own API group
	nrpclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	clusterNSLister   listers.ClusterNSLister
	clusterNSsSynced  cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewClusterNSController returns a new cluster controller
func NewClusterNSController(
	kubeclientset kubernetes.Interface,
	clusternsclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	clusterNSInformer informers.ClusterNSInformer) *ClusterNSController {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(nrpscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &ClusterNSController{
		kubeclientset:     kubeclientset,
		nrpclientset:      clusternsclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		clusterNSLister:   clusterNSInformer.Lister(),
		clusterNSsSynced:  clusterNSInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Clusters"),
		recorder:          recorder,
	}

	klog.Info("Setting up event handlers for clusterNS")
	// Set up an event handler for when Cluster resources change
	clusterNSInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueCluster,
		UpdateFunc: func(old, new interface{}) {
			// Don't do anything on an update
			//controller.enqueueCluster(new)
		},
		DeleteFunc: controller.enqueueClusterNSForDelete,
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Cluster resource then the handler will enqueue that Cluster resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.enqueueClusterNSForDelete,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *ClusterNSController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting ClusterNS controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.clusterNSsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Cluster resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *ClusterNSController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *ClusterNSController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Cluster resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Cluster resource
// with the current status of the resource.
func (c *ClusterNSController) syncHandler(key string) error {
	klog.Warning("In syncHandler")
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Cluster resource with this namespace/name
	clusterNS, err := c.clusterNSLister.ClusterNSs(namespace).Get(name)
	if err != nil {
		klog.Warningf("Error in get: %s - %s ", err.Error(), namespace)
		// If the ClusterNS resource no longer exists, it's been deleted, and we need
		// to clean things up
		if errors.IsNotFound(err) {
			if err = deleteClusterNS(clusterNS.Name, clusterNS.Spec.NS); err != nil {
				return err
			}
			return nil
		}

		return err
	}
	klog.Warning("Processing ClusterNS")

	// Process Cluster
	klog.Warning("Creating ClusterNS")
	if err = createClusterNSNamespace(clusterNS.Name); err != nil {
		return err
	}
	createClusterNSRoleBindings(clusterNS.Name, clusterNS.Spec.NS, clusterNS.Spec.NS)

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	c.recorder.Event(clusterNS, corev1.EventTypeNormal, ClusterNSSuccessSynced, ClusterNSMessageResourceSynced)
	return nil
}

// enqueueCluster takes a Cluster resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Cluster.
func (c *ClusterNSController) enqueueCluster(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Cluster resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Cluster resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *ClusterNSController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Cluster, we should not do anything more
		// with it.
		if ownerRef.Kind != "Cluster" {
			return
		}

		cluster, err := c.clusterNSLister.ClusterNSs(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s/%s' of cluster '%s'", object.GetNamespace(), object.GetName(), ownerRef.Name)
			return
		}

		c.enqueueCluster(cluster)
		return
	}
}

// enqueueClusterForDelete takes a deleted Cluster resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Network.
func (c *ClusterNSController) enqueueClusterNSForDelete(obj interface{}) {
	var key string
	var err error
	key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func deleteClusterNS(clusterNS string, clusterNamespace string) error {
	klog.Warning("Cluster Handler Delete")
	todoCtx := context.TODO()

	// TODO: get clusterNS and use had to delete namespaces
	kubeClient := getKubeClientSet()
	err := kubeClient.
		CoreV1().
		Namespaces().
		Delete(todoCtx, clusterNamespace, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Error deleting cluster namespace %s: %s", clusterNamespace, err.Error())
		return fmt.Errorf("While deleting namespace %s for %s, got error: %v",
			clusterNamespace,
			clusterNS,
			err)
	}
	return nil
}

func createClusterNSRoleBindings(clusterNSName string, svcAccount string, namespace string) {
	kubeClient := getKubeClientSet()
	roleBinding := rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterNSName,
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			Name:     "federation-cluster",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      svcAccount,
				Namespace: namespace,
			},
		},
	}
	_, err := kubeClient.
		RbacV1().
		RoleBindings(namespace).
		Create(context.TODO(), &roleBinding, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			klog.Errorf("Error creating federation-cluster rolebinding %s", err.Error())
		}
	}
}

func createClusterNSNamespace(namespace string) error {
	kubeClient := getKubeClientSet()
	namespaceSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	_, err := kubeClient.CoreV1().Namespaces().Create(context.TODO(), namespaceSpec, metav1.CreateOptions{})
	if err == nil || errors.IsAlreadyExists(err) {
		return nil
	} else {
		return fmt.Errorf("error creating namespace %s: %s", namespace, err.Error())
	}
}
