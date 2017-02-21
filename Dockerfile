FROM        quay.io/prometheus/busybox:latest
MAINTAINER  Michael Merrill <michael.merrill@vonage.com>

COPY job_exporter /bin/job_exporter

EXPOSE      9100
ENTRYPOINT  [ "/bin/job_exporter" ]