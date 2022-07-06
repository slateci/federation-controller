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
	"k8s.io/apimachinery/pkg/api/meta"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
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

	fedv1alpha2 "github.com/slateci/federation-controller/pkg/apis/federationcontroller/v1alpha2"
	clientset "github.com/slateci/federation-controller/pkg/generated/clientset/versioned"
	fedscheme "github.com/slateci/federation-controller/pkg/generated/clientset/versioned/scheme"
	informers "github.com/slateci/federation-controller/pkg/generated/informers/externalversions/federationcontroller/v1alpha2"
	listers "github.com/slateci/federation-controller/pkg/generated/listers/federationcontroller/v1alpha2"
)

const (
	// ClusterSuccessSynced is used as part of the Event 'reason' when a Cluster is synced
	ClusterSuccessSynced = "Synced"

	// ClusterSuccessDeleted is used as part of the Event 'reason' when a Cluster is synced
	ClusterSuccessDeleted = "Deleted"

	// ClusterErrResourceExists is used as part of the Event 'reason' when a Cluster fails
	// to sync due to a Deployment of the same name already existing.
	ClusterErrResourceExists = "ErrResourceExists"

	// ClusterMessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	ClusterMessageResourceExists = "Resource %q already exists and is not managed by Cluster"

	// ClusterMessageResourceSynced is the message used for an Event fired when a Cluster
	// is synced successfully
	ClusterMessageResourceSynced = "Cluster synced successfully"

	// ClusterMessageResourceDeleted is the message used for an Event fired when a Cluster
	// is deleted successfully
	ClusterMessageResourceDeleted = "Cluster deleted successfully"
)

// ClusterController is the controller implementation for Cluster resources
type ClusterController struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// nrpclientset is a clientset for our own API group
	nrpclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	clustersLister    listers.ClusterLister
	clustersSynced    cache.InformerSynced

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

// NewClusterController returns a new cluster controller
func NewClusterController(
	kubeclientset kubernetes.Interface,
	clusterclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	clusterInformer informers.ClusterInformer) *ClusterController {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(fedscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &ClusterController{
		kubeclientset:     kubeclientset,
		nrpclientset:      clusterclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		clustersLister:    clusterInformer.Lister(),
		clustersSynced:    clusterInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Clusters"),
		recorder:          recorder,
	}

	klog.V(4).Info("Setting up event handlers for Cluster")
	// Set up an event handler for when Cluster resources change
	clusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueCluster,
		UpdateFunc: func(old, new interface{}) {
			// Don't do anything on an update
			//controller.enqueueCluster(new)
		},
		DeleteFunc: controller.enqueueClusterForDelete,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *ClusterController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.V(4).Info("Starting Cluster controller")

	// Wait for the caches to be synced before starting workers
	klog.V(4).Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.clustersSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.V(4).Info("Starting workers")
	// Launch two workers to process Cluster resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.V(4).Info("Started workers")
	<-stopCh
	klog.V(4).Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *ClusterController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *ClusterController) processNextWorkItem() bool {
	klog.V(4).Info("In processNextWorkItem")
	obj, shutdown := c.workqueue.Get()
	klog.V(4).Info("Got workqueue item")

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
		klog.V(4).Infof("Got key %s to process", key)
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
		klog.V(4).Infof("Successfully synced '%s'", key)
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
func (c *ClusterController) syncHandler(key string) error {
	klog.V(4).Info("In syncHandler")
	// Convert the namespace/name string into a distinct namespace and name
	klog.V(4).Infof("Retrieving cluster key  - %s", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Cluster resource with this /name
	cluster, err := c.clustersLister.Clusters(namespace).Get(name)
	if err != nil {
		// ******** IMPORTANT ********
		// Possible get here if codegen is run and the modifications to generated
		// code to handle cluster CRDs haven't been made to codegened source code
		// If that's the case, then the following need to be modified:
		// generated lister for cluster, see https://github.com/slateci/federation-controller/blob/main/pkg/generated/listers/nrpcontroller/v1alpha1/cluster.go#L92
		// clientset: see https://github.com/slateci/federation-controller/blob/c1eb288efbb4c26e9d3870594a1dcf93169d2c36/pkg/generated/clientset/versioned/typed/nrpcontroller/v1alpha1/cluster.go
		klog.V(4).Infof("Error in get: %s - %s ", err.Error(), namespace)
		// If the Cluster resource no longer exists, it's been deleted and we need
		// to clean things up
		if errors.IsNotFound(err) {
			if err = deleteCluster(name, name); err != nil {
				return err
			}
			return nil
		}

		return err
	}
	klog.V(4).Info("Processing Cluster")

	// Process Cluster
	klog.V(4).Info("Creating namespace for Cluster")
	createdNamespace := createClusterNamespace(cluster.Name)
	klog.V(4).Info("Created namespace %s for Cluster", createdNamespace)
	klog.V(4).Info("Creating serviceAccount for Cluster")
	svcAcct := createServiceAccount(cluster.Name, createdNamespace)
	klog.V(4).Info("Creating role bindings for Cluster")
	createRoleBindings(cluster.Name, svcAcct, createdNamespace)

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Update cluster information
	cluster.Spec.Namespace = createdNamespace
	cluster.Spec.Organization = "slate"
	err = c.updateClusterStatus(cluster)
	if err != nil {
		klog.Errorf("Error updating cluster %s: %s", cluster.Name, err.Error())
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	c.recorder.Event(cluster, corev1.EventTypeNormal, ClusterSuccessSynced, ClusterMessageResourceSynced)
	return nil
}

func (c *ClusterController) updateClusterStatus(cluster *fedv1alpha2.Cluster) error {
	klog.V(4).Info("updateClusterStatus running")
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	clusterCopy := cluster.DeepCopy()
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Cluster resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.nrpclientset.FederationcontrollerV1alpha2().Clusters("").Update(context.TODO(), clusterCopy, metav1.UpdateOptions{})
	klog.V(4).Info("updateClusterStatus done")
	return err
}

// enqueueCluster takes a Cluster resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Cluster.
func (c *ClusterController) enqueueCluster(obj interface{}) {
	var key string
	var err error
	// Cluster has a Namespace field that is being used for to
	// store the namespace that the Cluster is using, this
	// interferes with the k8s introspection code so we can't
	// use default code such as:
	//if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
	//	utilruntime.HandleError(err)
	//	return
	//}
	// instead get object name and use that with default namespace
	metaParams, err := meta.Accessor(obj)
	if err != nil {
		klog.Errorf("object has no metaParams: %v", err)
	}
	key = "/" + metaParams.GetName()
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Cluster resource that 'owns' it. It does this by looking at the
// object's metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Cluster resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *ClusterController) handleObject(obj interface{}) {
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

		cluster, err := c.clustersLister.Clusters(object.GetNamespace()).Get(ownerRef.Name)
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
func (c *ClusterController) enqueueClusterForDelete(obj interface{}) {
	var key string
	var err error
	klog.V(4).Info("Deleting Cluster")
	key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
	klog.V(4).Info("Deleting Cluster done")
}

func createClusterNamespace(pattern string) string {
	kubeClient := getKubeClientSet()
	namespaceSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: pattern}}
	_, err := kubeClient.CoreV1().Namespaces().Create(context.TODO(), namespaceSpec, metav1.CreateOptions{})
	if err == nil {
		return pattern
	} else if errors.IsAlreadyExists(err) {
		// try successive namespace names until we get one that's unused
		for number := 0; ; number++ {
			tryName := fmt.Sprintf("%s-%d", pattern, number)
			namespaceSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tryName}}
			_, err = kubeClient.CoreV1().Namespaces().Create(context.TODO(), namespaceSpec, metav1.CreateOptions{})
			if err != nil {
				return tryName
			} else if errors.IsAlreadyExists(err) {
				continue
			} else {
				klog.Fatalf("Error creating namespace %s: %s", pattern, err.Error())
			}

		}
	} else {
		klog.Fatalf("Error creating namespace %s: %s", pattern, err.Error())
	}
	// should never get here
	klog.Fatal("Impossible condition while creating namespace")
	return ""
}

func createServiceAccount(clusterName string, namespace string) string {
	kubeClient := getKubeClientSet()
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
	}
	srvAcc, err := kubeClient.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), &serviceAccount, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			klog.Fatalf("Error creating service account %s", err.Error())
			return ""
		} else {
			// if the account already exists, it is using the clustername
			return clusterName
		}
	}
	return srvAcc.Name
}

func createRoleBindings(clusterName string, svcAccount string, namespace string) {
	kubeClient := getKubeClientSet()
	roleBinding := rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			Name:     "federation-cluster",
			APIGroup: "rbac.authorization.k8s.io",
		},
		Subjects: []rbac.Subject{
			{
				Kind: "ServiceAccount",
				Name: svcAccount,
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
	clusterRoleBinding := rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			Name:     "federation-cluster-global",
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
	_, err = kubeClient.
		RbacV1().
		ClusterRoleBindings().
		Create(context.TODO(), &clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			klog.Errorf("Error creating federation-cluster-global clusterrolebinding %s", err.Error())
		}
	}

}

func deleteCluster(clusterName string, clusterNamespace string) error {
	klog.V(4).Info("Cluster Handler Delete")
	todoCtx := context.TODO()

	kubeClient := getKubeClientSet()
	// Delete any namespaces created by ClusterNSs, need to do this before deleting the cluster namespace
	klog.V(4).Info("Getting ClusterNS for deletion")
	clusterClient := getClusterClientSet()
	clusterNSList, err := clusterClient.FederationcontrollerV1alpha2().ClusterNSs(clusterNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Error getting additional namespaces for %s: %s", clusterName, err.Error())
		return err
	}
	for _, item := range clusterNSList.Items {
		err := kubeClient.
			CoreV1().
			Namespaces().
			Delete(todoCtx, item.Spec.Namespace, metav1.DeleteOptions{})
		if err != nil {
			klog.Errorf("Error deleting cluster namespace %s: %s", item.Spec.Namespace, err.Error())
			return fmt.Errorf("while deleting namespace %s for %s, got error: %v",
				clusterName,
				clusterNamespace,
				err)
		}
	}

	err = kubeClient.
		CoreV1().
		Namespaces().
		Delete(todoCtx, clusterNamespace, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Error deleting cluster namespace %s: %s", clusterNamespace, err.Error())
		return fmt.Errorf("while deleting namespace %s for %s, got error: %v",
			clusterName,
			clusterNamespace,
			err)
	}
	err = kubeClient.
		RbacV1().
		RoleBindings(clusterName).
		Delete(todoCtx, clusterName, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Error deleting federation-cluster rolebinding %s", err.Error())
		return fmt.Errorf("while deleting rolebinding %s, got error: %v",
			clusterName,
			err)
	}
	err = kubeClient.
		CoreV1().
		ServiceAccounts(clusterNamespace).
		Delete(todoCtx, clusterName, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Error deleting service-account %s in namespace %s: %s",
			clusterName,
			clusterNamespace,
			err.Error())
		return fmt.Errorf("while deleting svc account %s, got error: %v",
			clusterName,
			err)
	}
	klog.V(4).Info("Deleted Cluster")
	return nil
}
