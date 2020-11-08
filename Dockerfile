FROM golang:1.13.15-alpine

WORKDIR /app

COPY . /app

RUN ["go", "build"]

EXPOSE 2775

EXPOSE 12775

ENTRYPOINT ["/app/smscsim"]

