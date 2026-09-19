FROM golang:1.26.3-alpine3.23 AS build

WORKDIR /app

COPY . /app

RUN apk upgrade --update \
    && apk add -U tzdata \
    && rm -rf /var/cache/apk/* \
    && go build

##########################################

FROM alpine:3.23.6

COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=build /app/smscsim /app/smscsim

ENTRYPOINT ["/app/smscsim"]

