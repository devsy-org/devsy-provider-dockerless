FROM alpine:3.21

RUN apk add --no-cache shadow shadow-subids iproute2

RUN useradd --shell /bin/sh --home /home/rootless --add-subids-for-system --uid 1000 --password "" rootless && \
    mkdir -p /home/rootless && chown -R rootless:rootless /home/rootless
