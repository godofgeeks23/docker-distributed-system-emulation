FROM python:3.12-alpine3.20

RUN apk add --no-cache iproute2 iputils curl

WORKDIR /app

COPY docker/probe/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY docker/probe/serve.py /app/serve.py

RUN chmod +x /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

