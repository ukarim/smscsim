# smscsim

Lightweight, zero-dependency and stupid SMSc simulator.

### Warning

* simulator implements only a small subset of the SMPP3.4 specification and supports only the following PDUs:
  - `bind_transmitter`, `bind_receiver`, `bind_transceiver`
  - `unbind`
  - `submit_sm`
  - `enquire_link`
  - `deliver_sm_resp`
* simulator does not perform PDU validation

### Usage

```bash
> go build
> ./smscsim
```

This will start smpp server on port _2775_ and web server on port _12775_.

### Delivery reports

If it was requested by _submit_sm_ packet, delivery receipt will be returned after 2 sec with a message state always set to _DELIVERED_.

### MO messages

MO messages can be triggered using a special web page http://localhost:12775.
MO message will be delivered to the selected smpp session using a _deliver_sm_ PDU.

### Env variables

* SMSC_PORT - override default smpp port
* WEB_PORT - override default web port

