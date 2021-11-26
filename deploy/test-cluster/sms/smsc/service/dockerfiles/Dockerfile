# -----  Builder -----
FROM golang:1.16 AS builder

# Setup private repo.
ENV GOPRIVATE=github.com/juphoon/*
ENV GOPROXY=https://goproxy.cn

# Copy source code to docker image.
COPY . /src
WORKDIR /src
RUN mkdir bin
RUN go build -o ./bin/smscsim

# ----- Container Image From Builder -----
FROM debian:buster-slim

# Disable intractive frontend for debian.
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends tzdata \
        ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

# Setup time zone.
RUN ln -fs /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
        && dpkg-reconfigure --frontend noninteractive tzdata

# Copy binary from pre-stage.
COPY --from=builder /src/bin /app

# --- Uncomment the codes below for testing locally ---
# COPY --from=builder /src/app/sms/tenant/service/config/.  /data/conf/.

WORKDIR /app

EXPOSE 2775
EXPOSE 12775

# Start the app.
CMD ["./smscsim"]