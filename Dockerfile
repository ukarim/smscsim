FROM dart:3.1.3-sdk AS build

WORKDIR /app

COPY . /app

RUN DEBIAN_FRONTEND=noninteractive apt-get install -y tzdata \
    && dart compile exe main.dart -o smscsim

##########################################

FROM scratch

# copy necessary libs
COPY --from=build /lib/x86_64-linux-gnu/libdl.so.2 /lib/x86_64-linux-gnu/libdl.so.2
COPY --from=build /lib/x86_64-linux-gnu/libpthread.so.0 /lib/x86_64-linux-gnu/libpthread.so.0
COPY --from=build /lib/x86_64-linux-gnu/libm.so.6 /lib/x86_64-linux-gnu/libm.so.6
COPY --from=build /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/libc.so.6
COPY --from=build /lib64/ld-linux-x86-64.so.2 /lib64/ld-linux-x86-64.so.2
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

# copy app itself
COPY --from=build /app/smscsim /app/smscsim

ENTRYPOINT ["/app/smscsim"]
