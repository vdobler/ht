FROM scratch
MAINTAINER Volker Dobler <dr.volker.dobler@gmail.com>
ADD zoneinfo.tar.gz /
ADD ca-certificates.crt /etc/ssl/certs/
ADD ht /
WORKDIR /wd
ENTRYPOINT ["/ht"]