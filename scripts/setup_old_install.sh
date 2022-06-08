#!/bin/bash

kubectl apply -f https://raw.githubusercontent.com/slateci/slate-client-server/develop/resources/federation-role.yaml
kubectl apply -f https://raw.githubusercontent.com/slateci/slate-client-server/develop/resources/federation-deployment.yaml
echo "Sleeping for 30 seconds for controller to initialize"
sleep 30
kubectl create namespace slate-system
sleep 10
echo "Sleeping for 10 seconds for namespace"
kubectl create -f  prod2-cluster.yaml
kubectl create -f prod2-clusternamespace.yaml
