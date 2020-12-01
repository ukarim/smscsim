FROM golang:1.13.15-alpine AS build

WORKDIR /app

COPY . /app

RUN apk upgrade --update \
    && apk add -U tzdata \
    && rm -rf /var/cache/apk/* \
    && go build

##########################################

FROM alpine:3.12.1

COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=build /app/smscsim /app/smscsim

ENTRYPOINT ["/app/smscsim"]

