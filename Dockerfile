FROM golang:1.25.0-bookworm AS build
ARG GIT_TAG
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y libudev-dev i2c-tools libpipewire-0.3-dev pkg-config
RUN mkdir -p /opt/LumenForge

WORKDIR /app
COPY . /app/LumenForge

WORKDIR /app/LumenForge
RUN if [ -n "$GIT_TAG" ]; then git checkout "$GIT_TAG"; fi
RUN go build .

FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install -y libpipewire-0.3-0 libudev-dev pciutils usbutils udev i2c-tools pulseaudio-utils && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /etc/modules-load.d
RUN echo 'KERNEL=="i2c-0", MODE="0600", OWNER="lumenforge"' | tee /etc/udev/rules.d/98-corsair-memory.rules
RUN echo "i2c-dev" | tee /etc/modules-load.d/i2c-dev.conf

COPY --from=build /app/LumenForge/LumenForge /opt/LumenForge/
COPY --from=build /app/LumenForge/database /opt/LumenForge/database
COPY --from=build /app/LumenForge/static /opt/LumenForge/static
COPY --from=build /app/LumenForge/web /opt/LumenForge/web
COPY --from=build /app/LumenForge/99-lumenforge.rules /etc/udev/rules.d/99-lumenforge.rules

WORKDIR /opt/LumenForge

ENTRYPOINT ["/opt/LumenForge/LumenForge"]