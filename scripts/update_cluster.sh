#!/bin/bash

echo "About to remove existing NRP controller deployment"
echo -n "Press y to conitinue: "
read resp
if [[ resp != "y" && resp != "Y" ]];
then
  echo "Aborting upgrade"
fi

# kubectl delete deployment nrp-controller
if [[ $? != 0 ]];
then
  echo  "Error while removing old NRP controller deployment"
  exit 1
fi

echo "Saving old Cluster CRDS in $PWD/crd"
mkdir crd
cd crd
kubectl get cluster -n slate-system -o yaml > cluster-crd-orig.yaml
sed 's/v1alpha1/v1alpha2/'  cluster-crd-orig.yaml | egrep -v '(creation|resource|uid| generation)' > cluster-crd-new.yaml

echo "Saving old ClusterNamespace CRDS in $PWD"
kubectl get clusternamespace -n slate-system -o yaml > clusternamespace-crd-orig.yaml
sed 's/v1alpha1/v1alpha2/' clusternamespace-crd-orig.yaml | sed 's/ClusterNamespace/ClusterNS/' | egrep -v '(creation|resource|uid| generation)' > clusternamespace-crd-new.yaml
echo "New CRDS created"
echo "Removing old CRDS from cluster"
# kubectl delete -f cluster-crd-orig.yaml
# kubectl delete -f clusternamespace-crd-orig.yaml
echo "Creating updated CRDS"
#kubectl create -f cluster-crd-new.yaml
#kubectl create -f clusternamespace-crd-new.yaml
cd ..

echo "Deploying new controller"
# kubectl delete deployment nrp-controller
# kutectl apply -f xxx/ssthapa/go/src/nrp-clone/upgrade-controller.yaml          

