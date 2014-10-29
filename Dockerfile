FROM scratch

MAINTAINER Wiley Fuller <wiley@alltheducks.com>

ADD docserver /docserver
ADD editor /editor
ADD ca-certificates.crt /etc/ssl/certs/ca-certificates.crt


