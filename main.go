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
	"flag"
	nrpcontrollerv1alpha1 "github.com/slateci/nrp-clone/pkg/apis/nrpcontroller/v1alpha2"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	//clientset "k8s.io/sample-controller/pkg/generated/clientset/versioned"
	//informers "k8s.io/sample-controller/pkg/generated/informers/externalversions"
	//"k8s.io/sample-controller/pkg/signals"

	clientset "github.com/slateci/nrp-clone/pkg/generated/clientset/versioned"
	informers "github.com/slateci/nrp-clone/pkg/generated/informers/externalversions"
	"github.com/slateci/nrp-clone/pkg/signals"
)

const controllerAgentName = "nrp-controller"
const controllerVersion = "0.3.1"

var (
	masterURL  string
	kubeconfig string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.V(4).Infof("Starting %s version %s", controllerAgentName, controllerVersion)
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	clusterClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building clientset: %s", err.Error())
	}

	apiextensionsClientSet, err := apiextensionsclient.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	klog.V(4).Info("Adding CRD")
	if err = nrpcontrollerv1alpha1.CreateClusterCRD(apiextensionsClientSet); err != nil {
		klog.Fatalf("Got error while creating Cluster CRD: %v", err.Error())
	}
	if err = nrpcontrollerv1alpha1.CreateNSCRD(apiextensionsClientSet); err != nil {
		klog.Fatalf("Got error while creating Cluster Namespace CRD: %v", err.Error())
	}
	klog.V(4).Info("Adding CRD done")

	klog.V(4).Info("Starting clusterController in goroutine")
	go func() {
		// set up signals so we handle the first shutdown signal gracefully
		stopCh := signals.SetupSignalHandler()
		kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
		// we just want to watch the default namespace
		clusterInformerFactory := informers.NewSharedInformerFactoryWithOptions(clusterClient,
			time.Second*30,
			informers.WithNamespace(""))
		klog.V(4).Info("Creating Cluster CRD controller")
		clusterController := NewClusterController(kubeClient, clusterClient,
			kubeInformerFactory.Apps().V1().Deployments(),
			clusterInformerFactory.Nrpcontroller().V1alpha1().Clusters())
		kubeInformerFactory.Start(stopCh)
		clusterInformerFactory.Start(stopCh)

		clusterController.Run(2, stopCh)
	}()
	// Sleep for a little bit to allow controller to initialize
	time.Sleep(10 * time.Second)
	klog.V(4).Info("Starting clusterNSController in goroutine")
	go func() {
		// set up signals so we handle the first shutdown signal gracefully
		stopCh := make(chan struct{})
		kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
		// we just want to watch the default namespace
		clusterNSInformerFactory := informers.NewSharedInformerFactory(clusterClient, time.Second*30)

		klog.V(4).Info("Creating ClusterNS CRD controller")
		clusterNSController := NewClusterNSController(kubeClient, clusterClient,
			clusterNSInformerFactory.Nrpcontroller().V1alpha1().ClusterNSs())

		// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
		// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
		kubeInformerFactory.Start(stopCh)
		clusterNSInformerFactory.Start(stopCh)

		clusterNSController.Run(2, stopCh)
	}()
	select {}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
