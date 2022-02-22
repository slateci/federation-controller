FROM debian:bullseye-slim
ADD nrp-clone /
RUN mv /nrp-clone /nrp-controller
CMD ["/nrp-controller"]
EXPOSE 80
