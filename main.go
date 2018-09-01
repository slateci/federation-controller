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
						clientset.RbacV1().Roles(clusterns.Name).Create()
						clientset.RbacV1().RoleBindings(clusterns.Name).Create(&rbacv1.RoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name: "clusterrolebinding",
							},

							Rules: []rbacv1.PolicyRule{
								{
									APIGroups:     []string{"extensions"},
									Verbs:         []string{"use"},
									Resources:     []string{"podsecuritypolicies"},
									ResourceNames: []string{"nautilususerpolicy"},
								},
							},
						})
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
	try_name := fmt.Sprintf("%s-%d", pattern, num)
	for _, err := clientset.CoreV1().Namespaces().Get(try_name, metav1.GetOptions{}); err == nil {
		num += 1
		try_name := fmt.Sprintf("%s-%d", pattern, num)
	}
	return try_name
}