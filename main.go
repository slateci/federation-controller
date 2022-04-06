package main

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"log"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	nrpapi "github.com/slateci/nrp-clone/pkg/apis/nrpapi/v1alpha1"

	apiextcs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"

	v1 "k8s.io/api/core/v1"
)

var clientset *kubernetes.Clientset
var clustcrdclient *nrpapi.ClusterCrdClient
var clustnscrdclient *nrpapi.ClusterNSCrdClient

var clusterControllerHandlers = cache.ResourceEventHandlerFuncs{
	AddFunc: func(obj interface{}) {
		log.Println("Cluster Handler Add")
		cluster, ok := obj.(*nrpapi.Cluster)
		if !ok {
			log.Printf("Expected Cluster but other received %#v", obj)
			return
		}
		todoCtx := context.TODO()

		if cluster.Spec.Namespace == "" {
			clusterNamespace := v1.Namespace{
				ObjectMeta: metaV1.ObjectMeta{
					Name: findFreeNamespace(todoCtx, cluster.Name),
				},
			}
			clusterns, err := clientset.CoreV1().Namespaces().Create(todoCtx, &clusterNamespace, metaV1.CreateOptions{})
			if err != nil {
				log.Printf("Error creating cluster namespace %s", err.Error())
				return
			}
			serviceAccount := v1.ServiceAccount{
				ObjectMeta: metaV1.ObjectMeta{
					Name:      cluster.Name,
					Namespace: clusterns.Name,
				},
			}
			srvAcc, err := clientset.CoreV1().ServiceAccounts(clusterns.Name).Create(todoCtx, &serviceAccount, metaV1.CreateOptions{})
			if err != nil {
				log.Printf("Error creating service account %s", err.Error())
				return
			}
			roleBinding := rbacv1.RoleBinding{
				ObjectMeta: metaV1.ObjectMeta{
					Name: cluster.Name,
				},
				RoleRef: rbacv1.RoleRef{
					Kind:     "ClusterRole",
					Name:     "federation-cluster",
					APIGroup: "rbac.authorization.k8s.io",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind: "ServiceAccount",
						Name: srvAcc.Name,
					},
				},
			}
			_, err = clientset.RbacV1().RoleBindings(clusterns.Name).Create(todoCtx, &roleBinding, metaV1.CreateOptions{})
			if err != nil {
				log.Printf("Error creating federation-cluster rolebinding %s", err.Error())
			}
			clusterRoleBinding := rbacv1.ClusterRoleBinding{
				ObjectMeta: metaV1.ObjectMeta{
					Name: cluster.Name,
				},
				RoleRef: rbacv1.RoleRef{
					Kind:     "ClusterRole",
					Name:     "federation-cluster-global",
					APIGroup: "rbac.authorization.k8s.io",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      srvAcc.Name,
						Namespace: clusterns.Name,
					},
				},
			}
			_, err = clientset.RbacV1().ClusterRoleBindings().Create(todoCtx, &clusterRoleBinding, metaV1.CreateOptions{})
			if err != nil {
				log.Printf("Error creating federation-cluster-global clusterrolebinding %s", err.Error())
			}

			cluster.Spec.Namespace = clusterns.Name
			if _, err := clustcrdclient.Update(todoCtx, cluster); err != nil {
				log.Printf("Error updating cluster %s ns %s", cluster.Name, err.Error())
			}
		}
	},
	DeleteFunc: func(obj interface{}) {
		log.Println("Cluster Handler Delete")
		cluster, ok := obj.(*nrpapi.Cluster)
		todoCtx := context.TODO()

		if !ok {
			log.Printf("Expected Cluster but other received %#v", obj)
			return
		}
		if cluster.Spec.Namespace != "" {
			if clusterNamespaces, err := clustnscrdclient.List(todoCtx, cluster.Spec.Namespace, metaV1.ListOptions{}); err == nil {
				for _, clusterNs := range clusterNamespaces.Items {
					if err := clientset.CoreV1().Namespaces().Delete(todoCtx, clusterNs.Name, metaV1.DeleteOptions{}); err != nil {
						fmt.Printf("Error deleting clusternamespace %s %s", clusterNs.Name, err.Error())
					}
				}
				if err := clientset.CoreV1().Namespaces().Delete(todoCtx, cluster.Spec.Namespace, metaV1.DeleteOptions{}); err != nil {
					fmt.Printf("Error deleting cluster namespace %s %s", cluster.Spec.Namespace, err.Error())
				}
			}

		}
	},
	UpdateFunc: func(oldObj, newObj interface{}) {
		log.Println("Cluster Handler Update")
		_, ok := oldObj.(*nrpapi.Cluster)
		if !ok {
			log.Printf("Expected Cluster but other received %#v", oldObj)
			return
		}
		_, ok = newObj.(*nrpapi.Cluster)
		if !ok {
			log.Printf("Expected Cluster but other received %#v", newObj)
			return
		}
	},
}

var clusterNSControllerHandlers = cache.ResourceEventHandlerFuncs{
	AddFunc: func(obj interface{}) {
		log.Println("ClusterNS Handler Add")
		clusterNs, ok := obj.(*nrpapi.ClusterNamespace)

		if !ok {
			log.Printf("Expected ClusterNamespace but other received %#v", obj)
			return
		}
		todoCtx := context.TODO()

		namespace := v1.Namespace{
			ObjectMeta: metaV1.ObjectMeta{
				Name: clusterNs.Name,
			},
		}
		clusterns, err := clientset.CoreV1().Namespaces().Create(todoCtx, &namespace, metaV1.CreateOptions{})
		if err != nil {
			log.Printf("Error creating cluster namespace %s", err.Error())
			return
		}
		roleBinding := rbacv1.RoleBinding{
			ObjectMeta: metaV1.ObjectMeta{
				Name: clusterNs.Name,
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     "federation-cluster",
				APIGroup: "rbac.authorization.k8s.io",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      clusterNs.Namespace,
					Namespace: clusterNs.Namespace,
				},
			},
		}
		_, err = clientset.RbacV1().RoleBindings(clusterns.Name).Create(todoCtx, &roleBinding, metaV1.CreateOptions{})
		if err != nil {
			log.Printf("Error creating cluster rolebinding %s", err.Error())
			return
		}

	},
	DeleteFunc: func(obj interface{}) {
		clusterNs, ok := obj.(*nrpapi.ClusterNamespace)
		if !ok {
			log.Printf("Expected ClusterNamespace but other received %#v", obj)
			return
		}
		todoCtx := context.TODO()
		if err := clientset.CoreV1().Namespaces().Delete(todoCtx, clusterNs.Name, metaV1.DeleteOptions{}); err != nil {
			log.Printf("Error deleting namespace for cluster: %s", err.Error())
			return
		}
	},
	UpdateFunc: func(oldObj, newObj interface{}) {
		log.Println("ClusterNS Handler Update")
		_, ok := oldObj.(*nrpapi.ClusterNamespace)
		if !ok {
			log.Printf("Expected Cluster but other received %#v", oldObj)
			return
		}
		_, ok = newObj.(*nrpapi.ClusterNamespace)
		if !ok {
			log.Printf("Expected Cluster but other received %#v", newObj)
			return
		}
	},
}

func main() {
	log.Println("Starting nrp-clone controller v0.1.19")
	ctx := context.Background()

	k8sconfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed to do inclusterconfig: " + err.Error())
		return
	}

	// Create a new clientset which include our CRD schema
	log.Println("creating CRD ")
	crdcs, scheme, err := nrpapi.NewClient(k8sconfig)
	if err != nil {
		log.Printf("Error creating CRD client: %s", err.Error())
		log.Printf("Error creating CRD client: %s", err.Error())
	}

	clustcrdclient = nrpapi.MakeClusterCrdClient(crdcs, scheme, "default")
	clustnscrdclient = nrpapi.MakeClusterNSCrdClient(crdcs, scheme)

	clientset, err = kubernetes.NewForConfig(k8sconfig)
	if err != nil {
		log.Printf("Error creating client: %s", err.Error())
	}

	//log.Printf("Testing query")
	//result := nrpapi.ClusterList{}
	//testerr := crdcs.
	//	Get().
	//	Resource("clusters").
	//	Do(ctx).
	//	Into(&result)
	//if testerr != nil {
	//	log.Printf("Got error %s", testerr)
	//}
	//log.Printf("result: %#v", result)
	//log.Printf("query tested")
	//
	//log.Printf("Testing ns query")
	//nsresult := nrpapi.ClusterNamespaceList{}
	//nstesterr := crdcs.
	//	Get().
	//	Resource("clusters").
	//	Do(ctx).
	//	Into(&result)
	//if nstesterr != nil {
	//	log.Printf("Got error %s", testerr)
	//}
	//log.Printf("result: %#v", nsresult)
	//log.Printf("ns query tested")

	go func() {
		log.Print("Getting CRD")
		GetCrd(ctx)
	}()

	select {}

}

func GetCrd(ctx context.Context) {
	k8sconfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed to do inclusterconfig: " + err.Error())
		return
	}
	log.Println("newForConfig")
	crdclientset, err := apiextcs.NewForConfig(k8sconfig)
	if err != nil {
		panic(err.Error())
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	log.Println("CreateClusterCRD")
	if err := nrpapi.CreateClusterCRD(timeoutCtx, crdclientset); err != nil {
		log.Printf("Error creating CRD: %s", err.Error())
	}
	log.Println("CreateNSCRD")
	if err := nrpapi.CreateNSCRD(ctx, crdclientset); err != nil {
		log.Printf("Error creating NS CRD: %s", err.Error())
	}

	// Wait for the CRD to be created before we use it (only needed if it's a new one)
	time.Sleep(3 * time.Second)

	//log.Printf("clientset test")
	//clusterList, err := clustcrdclient.List(context.TODO(), "default", metaV1.ListOptions{})
	//clusterNSList, err := clustnscrdclient.List(context.TODO(), "default", metaV1.ListOptions{})
	//log.Printf("list: %#v", clusterList)
	//log.Printf("NS list: %#v", clusterNSList)
	//log.Printf("clientset test done")

	log.Println("Creating NewListWatch Cluster")
	//_, clusterController := cache.NewInformer(
	//	clustcrdclient.NewListWatch(),
	//	&nrpapi.Cluster{},
	//	time.Minute*1,
	//	clusterControllerHandlers,
	//)
	_, clusterController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metaV1.ListOptions) (result runtime.Object, err error) {
				log.Printf("in cluster ListFunc")
				clusterListCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				return clustcrdclient.List(clusterListCtx, "default", lo)
			},
			WatchFunc: func(lo metaV1.ListOptions) (watch.Interface, error) {
				log.Printf("in cluster WatchFunc")
				clusterWatchCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				return clustcrdclient.Watch(clusterWatchCtx, "default", lo)
			},
			DisableChunking: true,
		},
		&nrpapi.Cluster{},
		time.Minute*1,
		clusterControllerHandlers,
	)

	log.Println("Creating NewListWatch NS")
	_, clusterNSController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metaV1.ListOptions) (result runtime.Object, err error) {
				log.Printf("in NS ListFunc")
				nsListCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				return clustnscrdclient.List(nsListCtx, "default", lo)
			},
			WatchFunc: func(lo metaV1.ListOptions) (watch.Interface, error) {
				log.Printf("in NS WatchFunc")
				nsWatchCtx := context.TODO()
				//defer cancel()
				return clustnscrdclient.Watch(nsWatchCtx, "default", lo)
			},
			DisableChunking: true,
		},
		&nrpapi.ClusterNamespace{},
		time.Minute*1,
		clusterNSControllerHandlers,
	)

	go clusterController.Run(wait.NeverStop)
	go clusterNSController.Run(wait.NeverStop)

	log.Println("looking for controllers")
	// Wait forever
	select {
	case <-ctx.Done():
		switch ctx.Err() {
		case context.DeadlineExceeded:
			fmt.Println("context timeout exceeded")
		case context.Canceled:
			fmt.Println("context cancelled by force. whole process is complete")
		}
	}
}

func findFreeNamespace(ctx context.Context, pattern string) string {
	timeout1, cancel1 := context.WithTimeout(ctx, 30*time.Second)
	defer cancel1()
	if _, err := clientset.CoreV1().Namespaces().Get(timeout1, pattern, metaV1.GetOptions{}); err != nil {
		return pattern
	}
	num := 0
	tryName := fmt.Sprintf("%s-%d", pattern, num)
	var err error = nil
	timeout2, cancel2 := context.WithTimeout(ctx, 30*time.Second)
	defer cancel2()
	for ; err != nil; _, err = clientset.CoreV1().Namespaces().Get(timeout2, tryName, metaV1.GetOptions{}) {
		num += 1
		tryName = fmt.Sprintf("%s-%d", pattern, num)
	}
	return tryName
}
