package main

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"time"
	"k8s.io/client-go/tools/cache"
)

var clientset *kubernetes.Clientset

func main() {
	k8sconfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed to do inclusterconfig: " + err.Error())
		return
	}

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

	if err := nautilusapi.CreateCRD(crdclientset); err != nil {
		log.Printf("Error creating CRD: %s", err.Error())
	}

	// Wait for the CRD to be created before we use it (only needed if its a new one)
	time.Sleep(3 * time.Second)

	_, controller := cache.NewInformer(
		crdclient.NewListWatch(),
		&nautilusapi.PRPUser{},
		time.Minute*5,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				user, ok := obj.(*nautilusapi.PRPUser)
				if !ok {
					log.Printf("Expected PRPUser but other received %#v", obj)
					return
				}

				updateClusterUserPrivileges(user)
			},
			DeleteFunc: func(obj interface{}) {
				user, ok := obj.(*nautilusapi.PRPUser)
				if !ok {
					log.Printf("Expected PRPUser but other received %#v", obj)
					return
				}

				if rb, err := clientset.Rbac().ClusterRoleBindings().Get("nautilus-cluster-user", metav1.GetOptions{}); err == nil {
					allSubjects := []rbacv1.Subject{} // to filter the user, in case we need to delete one

					found := false
					for _, subj := range rb.Subjects {
						if subj.Name == user.Spec.UserID {
							found = true
						} else {
							allSubjects = append(allSubjects, subj)
						}
					}
					if found {
						rb.Subjects = allSubjects
						if _, err := clientset.Rbac().ClusterRoleBindings().Update(rb); err != nil {
							log.Printf("Error updating user %s: %s", user.Name, err.Error())
						}
					}
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldUser, ok := oldObj.(*nautilusapi.PRPUser)
				if !ok {
					log.Printf("Expected PRPUser but other received %#v", oldObj)
					return
				}
				newUser, ok := newObj.(*nautilusapi.PRPUser)
				if !ok {
					log.Printf("Expected PRPUser but other received %#v", newObj)
					return
				}
				if oldUser.Spec.Role != newUser.Spec.Role {
					updateClusterUserPrivileges(newUser)
				}
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)

	// Wait forever
	select {}
}