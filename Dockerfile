FROM scratch
ADD nrp-clone /nrp-controller
CMD ["/nrp-controller"]
EXPOSE 80
