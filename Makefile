default: buildrelease

buildgo:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix "static" .

builddocker:
	docker build -t registry.gitlab.com/ucsd-prp/nrp-controller:latest .

pushdocker:
	docker push registry.gitlab.com/ucsd-prp/nrp-controller

cleanup:
	rm nrp-controller

buildrelease: buildgo builddocker pushdocker cleanup

restartpod:
	kubectl delete pods --selector=k8s-app=nrp-controller -n kube-system

