FROM alpine:3.20

RUN apk add --no-cache iproute2 iptables

COPY docker/router/entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

