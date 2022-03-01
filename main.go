package main

import (
	"context"
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
	ctx := context.Background()

	k8sconfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed to do inclusterconfig: " + err.Error())
		return
	}

	// Create a new clientset which include our CRD schema
	crdcs, scheme, err := nrpapi.NewClient(k8sconfig)
	if err != nil {
		log.Printf("Error creating CRD client: %s", err.Error())
	}

	clustcrdclient = nrpapi.MakeClusterCrdClient(crdcs, scheme, "default")
	clustnscrdclient = nrpapi.MakeClusterNSCrdClient(crdcs, scheme)

	clientset, err = kubernetes.NewForConfig(k8sconfig)
	if err != nil {
		log.Printf("Error creating client: %s", err.Error())
	}

	go func() {
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

	crdclientset, err := apiextcs.NewForConfig(k8sconfig)
	if err != nil {
		panic(err.Error())
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := nrpapi.CreateClusterCRD(timeoutCtx, crdclientset); err != nil {
		log.Printf("Error creating CRD: %s", err.Error())
	}

	if err := nrpapi.CreateNSCRD(ctx, crdclientset); err != nil {
		log.Printf("Error creating CRD: %s", err.Error())
	}

	// Wait for the CRD to be created before we use it (only needed if it's a new one)
	time.Sleep(3 * time.Second)

	_, clusterController := cache.NewInformer(
		clustcrdclient.NewListWatch(),
		&nrpapi.Cluster{},
		time.Minute*1,
		clusterControllerHandlers,
	)

	_, clusterNSController := cache.NewInformer(
		clustnscrdclient.NewListWatch(""),
		&nrpapi.ClusterNamespace{},
		time.Minute*1,
		clusterNSControllerHandlers,
	)

	stop := make(chan struct{})
	go clusterController.Run(stop)
	go clusterNSController.Run(stop)

	// Wait forever
	select {}
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
