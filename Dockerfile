FROM golang:1.13.15-alpine AS build

WORKDIR /app

COPY . /app

RUN ["go", "build"]


##########################################

FROM alpine:3.12.1

COPY --from=build /app/smscsim /app/smscsim

ENTRYPOINT ["/app/smscsim"]

