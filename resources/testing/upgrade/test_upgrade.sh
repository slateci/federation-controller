#!/bin/bash

echo "Installing old nrp-controller"
kubectl apply -f federation-role.yaml
kubectl apply -f federation-deployment.yaml
sleep 30
echo "Creating CRDs"
kubectl apply -f old-cluster.yaml
sleep 30
kubectl apply -f old-clusternamespace.yaml
#sleep 30
#echo "Updating controller"
#kubectl apply -f https://raw.githubusercontent.com/slateci/federation-controller/rename_and_upgrade_support/resources/installation/upgrade-controller-debug.yaml
