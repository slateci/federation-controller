## Upgrading to new Slate NRP controller
To upgrade the NRP controller from the prior version, do the following

* Run `kubectl get clusternamespace -A -o yaml > clusternamespaces.yaml`
* Edit `clusternamespaces.yaml` and make the following changes to each entry:
  * Change `apiversion` to `nrp-nautilus.io/v1alpha2`
  * Change `kind` from `ClusterNamespace` to `ClusterNS`
  * Delete the `resourceVersion`, `uid`, `generation`, and `creationTimestamp` fields
* Delete the existing `nrp-controller` deployment
* Deploy the new nrp controller using `kubectl apply -f upgrade.yaml`
* Add the new `ClusterNS` entries using `kubectl create -f clusternamespaces.yaml`
