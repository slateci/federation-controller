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
var crdclient *nrpapi.CrdClient

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

	crdclient = nrpapi.MakeCrdClient(crdcs, scheme, "default")

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

	if err := nrpapi.CreateCRD(crdclientset); err != nil {
		log.Printf("Error creating CRD: %s", err.Error())
	}

	// Wait for the CRD to be created before we use it (only needed if its a new one)
	time.Sleep(3 * time.Second)

	_, controller := cache.NewInformer(
		crdclient.NewListWatch(),
		&nrpapi.Cluster{},
		time.Minute*5,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cluster, ok := obj.(*nrpapi.Cluster)
				if !ok {
					log.Printf("Expected Cluster but other received %#v", obj)
					return
				}
				if cluster.Namespace == "" {
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
							clientset.RbacV1().RoleBindings(clusterns.Name).Create(&rbacv1.RoleBinding{
								ObjectMeta: metav1.ObjectMeta{
									Name: cluster.Name,
								},
								RoleRef: rbacv1.RoleRef{
									Kind: "ClusterRole",
									Name: "admin",
									APIGroup: "rbac.authorization.k8s.io",
								},
								Subjects: []rbacv1.Subject{
									rbacv1.Subject{
										Kind: "ServiceAccount",
										Name: srvAcc.Name,
									},
								},
							})
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
				_, ok := obj.(*nrpapi.Cluster)
				if !ok {
					log.Printf("Expected Cluster but other received %#v", obj)
					return
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

	stop := make(chan struct{})
	go controller.Run(stop)

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