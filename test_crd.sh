#!/bin/bash

kubectl create -f cluster-test.yaml
sleep 10
echo "Check for namespace creation"
read foo
kubectl create -f  clusterns-test.yaml
sleep 10
echo "Check for namespace creation"
read foo
kubectl delete cluster cluster1
sleep 10
echo "Check for namespace deletion"
read foo
echo "Completed"
