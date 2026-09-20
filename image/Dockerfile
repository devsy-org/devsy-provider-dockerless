FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk add --no-cache shadow shadow-subids iproute2

RUN useradd --shell /bin/sh --home /home/rootless --add-subids-for-system --uid 1000 --password "" rootless && \
    mkdir -p /home/rootless && chown -R rootless:rootless /home/rootless
