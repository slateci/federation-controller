default: buildrelease

buildgo:
	go build 

builddocker:
	docker build -t registry.gitlab.com/ucsd-prp/nrp-controller:latest .

pushminikube: 
	minikube image  build . -t nrp-controller:latest
	
pushdocker:
	docker push registry.gitlab.com/ucsd-prp/nrp-controller

cleanup:
	rm nrp-clone

buildrelease: buildgo builddocker pushdocker cleanup

buildminikube: buildgo pushminikube
