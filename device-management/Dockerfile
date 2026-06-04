FROM alpine:3.23

RUN apk add --no-cache ca-certificates wget

# Create app user
RUN addgroup -S taksa && adduser -S taksa -G taksa

# /data is world-writable for host volume mount compatibility (docker-compose dev)
RUN mkdir -p /data /home/taksa && chown -R taksa:taksa /home/taksa && chmod 777 /data

WORKDIR /home/taksa

# Binary and configs are pre-built by 'make build' before docker build
COPY --chown=taksa:taksa bin/device-management ./device-management
COPY --chown=taksa:taksa configs ./configs

USER taksa

# Use GET (-O /dev/null), not wget --spider: spider sends HEAD and Kratos only registers GET for /health → 404.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 -O /dev/null http://localhost:8000/health || exit 1

EXPOSE 8000 9000

CMD ["/home/taksa/device-management", "-conf", "/home/taksa/configs"]
