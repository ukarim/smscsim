# smscsim

![Run tests](https://github.com/ukarim/smscsim/workflows/run-tests/badge.svg)

Lightweight, zero-dependency and stupid SMSc simulator.

### Usage

1) Use prebuild docker image (from hub.docker.com)

```
docker run -p 2775:2775 -p 12775:12775 ukarim/smscsim
```

2) Build from sources (need golang compiler)

```
go build
./smscsim
```

then, just configure your smpp client to connect to `localhost:2775`

### Features

#### Delivery reports (DLR)

If it was requested by _submit_sm_ packet, delivery receipt will be returned
after 2 sec with a message state always set to _DELIVERED_.

#### MO messages

Mobile originated messages (from `smsc` to `smpp client`) can be sent using
special web page available at `http://localhost:12775` . MO message will be
delivered to the selected smpp session using a _deliver_sm_ PDU.

### Warning

* simulator implements only a small subset of the SMPP3.4 specification and supports only the following PDUs:
  - `bind_transmitter`, `bind_receiver`, `bind_transceiver`
  - `unbind`
  - `submit_sm`
  - `enquire_link`
  - `deliver_sm_resp`
* simulator does not perform PDU validation

### Env variables

* SMSC_PORT - override default smpp port
* WEB_PORT - override default web port
* FAILED_SUBMITS - if this is set to true, submit_sm requests will fail
  - for submit_sm with even sequence number smscsim will return submit_sm_resp with command_status set to 0x00000008 (System Error)
  - for submit_sm with odd sequence number smscsim will return DLR with UNDELIVERABLE message state
