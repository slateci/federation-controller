package main

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"time"
	"k8s.io/client-go/tools/cache"
	"log"
	rbacv1 "k8s.io/api/rbac/v1"

	nrpapi "gitlab.com/ucsd-prp/nrp-controller/pkg/apis/nrp-nautilus.io/v1alpha1"
	apiextcs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/api/core/v1"
	"fmt"
)

var clientset *kubernetes.Clientset
var clustcrdclient *nrpapi.ClusterCrdClient
var clustnscrdclient *nrpapi.ClusterNSCrdClient

func main() {
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

	go func(){
		GetCrd()
	}()

	select {}

}

func GetCrd() {
	k8sconfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed to do inclusterconfig: " + err.Error())
		return
	}

	crdclientset, err := apiextcs.NewForConfig(k8sconfig)
	if err != nil {
		panic(err.Error())
	}

	if err := nrpapi.CreateClusterCRD(crdclientset); err != nil {
		log.Printf("Error creating CRD: %s", err.Error())
	}

	if err := nrpapi.CreateNSCRD(crdclientset); err != nil {
		log.Printf("Error creating CRD: %s", err.Error())
	}

	// Wait for the CRD to be created before we use it (only needed if its a new one)
	time.Sleep(3 * time.Second)

	_, clusterController := cache.NewInformer(
		clustcrdclient.NewListWatch(),
		&nrpapi.Cluster{},
		time.Minute*1,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cluster, ok := obj.(*nrpapi.Cluster)
				if !ok {
					log.Printf("Expected Cluster but other received %#v", obj)
					return
				}
				if cluster.Spec.Namespace == "" {
					if clusterns, err := clientset.CoreV1().Namespaces().Create(&v1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: findFreeNamespace(cluster.Name),
						},
					}); err == nil {
						if srvAcc, err := clientset.CoreV1().ServiceAccounts(clusterns.Name).Create(&v1.ServiceAccount{
							ObjectMeta: metav1.ObjectMeta{
								Name: cluster.Name,
								Namespace: clusterns.Name,
							},
						}); err == nil {
							if _, err := clientset.RbacV1().RoleBindings(clusterns.Name).Create(&rbacv1.RoleBinding{
								ObjectMeta: metav1.ObjectMeta{
									Name: cluster.Name,
								},
								RoleRef: rbacv1.RoleRef{
									Kind: "ClusterRole",
									Name: "federation-cluster",
									APIGroup: "rbac.authorization.k8s.io",
								},
								Subjects: []rbacv1.Subject{
									rbacv1.Subject{
										Kind: "ServiceAccount",
										Name: srvAcc.Name,
									},
								},
							}); err != nil {
								log.Printf("Error creating federation-cluster rolebinding %s", err.Error())
							}

							cluster.Spec.Namespace = clusterns.Name
							if _, err := clustcrdclient.Update(cluster); err != nil {
								log.Printf("Error updating cluster %s ns %s", cluster.Name, err.Error())
							}
						} else {
							log.Printf("Error creating service account %s", err.Error())
							return
						}
					} else {
						log.Printf("Error creating cluster namespace %s", err.Error())
						return
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				cluster, ok := obj.(*nrpapi.Cluster)
				if !ok {
					log.Printf("Expected Cluster but other received %#v", obj)
					return
				}
				if cluster.Spec.Namespace != "" {
					if clusterNamespaces, err := clustnscrdclient.List(cluster.Spec.Namespace, metav1.ListOptions{}); err == nil {
						for _, clusterNs := range clusterNamespaces.Items {
							if err := clientset.CoreV1().Namespaces().Delete(clusterNs.Name, &metav1.DeleteOptions{}); err != nil {
								fmt.Printf("Error deleting clusternamespace %s %s", clusterNs.Name, err.Error())
							}
						}
						if err := clientset.CoreV1().Namespaces().Delete(cluster.Spec.Namespace, &metav1.DeleteOptions{}); err != nil {
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
		},
	)


	_, clusterNSController := cache.NewInformer(
		clustnscrdclient.NewListWatch(""),
		&nrpapi.ClusterNamespace{},
		time.Minute*1,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				clusterNs, ok := obj.(*nrpapi.ClusterNamespace)
				if !ok {
					log.Printf("Expected ClusterNamespace but other received %#v", obj)
					return
				}
				if clusterns, err := clientset.CoreV1().Namespaces().Create(&v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterNs.Name,
					},
				}); err == nil {
					clientset.RbacV1().RoleBindings(clusterns.Name).Create(&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: clusterNs.Name,
						},
						RoleRef: rbacv1.RoleRef{
							Kind: "ClusterRole",
							Name: "federation-cluster",
							APIGroup: "rbac.authorization.k8s.io",
						},
						Subjects: []rbacv1.Subject{
							rbacv1.Subject{
								Kind: "ServiceAccount",
								Name: clusterNs.Namespace,
								Namespace: clusterNs.Namespace,
							},
						},
					})
				} else {
					log.Printf("Error creating cluster namespace %s", err.Error())
					return
				}
			},
			DeleteFunc: func(obj interface{}) {
				clusterNs, ok := obj.(*nrpapi.ClusterNamespace)
				if !ok {
					log.Printf("Expected ClusterNamespace but other received %#v", obj)
					return
				}

				if err := clientset.CoreV1().Namespaces().Delete(clusterNs.Name, &metav1.DeleteOptions{}); err == nil {
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
		},
	)

	stop := make(chan struct{})
	go clusterController.Run(stop)
	go clusterNSController.Run(stop)

	// Wait forever
	select {}
}

func findFreeNamespace(pattern string) string {
	if _, err := clientset.CoreV1().Namespaces().Get(pattern, metav1.GetOptions{}); err != nil {
		return pattern
	}
	num := 0
	tryName := fmt.Sprintf("%s-%d", pattern, num)
	var err error = nil
	for ; err != nil; _, err = clientset.CoreV1().Namespaces().Get(tryName, metav1.GetOptions{}) {
		num += 1
		tryName = fmt.Sprintf("%s-%d", pattern, num)
	}
	return tryName
}