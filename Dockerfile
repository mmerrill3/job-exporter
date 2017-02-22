FROM scratch
MAINTAINER  Michael Merrill <michael.merrill@vonage.com>
COPY job-exporter /
VOLUME /tmp

ENTRYPOINT ["/job-exporter", "--port=8080"]

EXPOSE 8080