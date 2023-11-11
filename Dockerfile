FROM eclipse-temurin:21.0.1_12-jdk-alpine AS build

WORKDIR /app

COPY . /app

RUN apk upgrade --update \
    && apk add -U tzdata \
    && cd src\
    && javac module-info.java \
    && javac smscsim/*.java \
    && jar --create --file smscsim.jar module-info.class smscsim/*.class \
    && jlink --module-path smscsim.jar --add-modules smscsim --output build_out --launcher smscsim=smscsim/smscsim.Main

##########################################

FROM alpine:3.18

COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=build /app/src/build_out /app

ENTRYPOINT ["/app/bin/smscsim"]
