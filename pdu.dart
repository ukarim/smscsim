import 'dart:typed_data';

import 'util.dart';

enum Cmd {
  genericNack(0x80000000),
  bindReceiver(0x00000001),
  bindReceiverResp(0x80000001),
  bindTransmitter(0x00000002),
  bindTransmitterResp(0x80000002),
  bindTransceiver(0x00000009),
  bindTransceiverResp(0x80000009),
  submitSm(0x00000004),
  submitSmResp(0x80000004),
  deliverSm(0x00000005),
  deliverSmResp(0x80000005),
  unbind(0x00000006),
  unbindResp(0x80000006),
  enquireLink(0x00000015),
  enquireLinkResp(0x80000015),
  unknown(0x00000000);

  final int cmdId;

  const Cmd(this.cmdId);

  Cmd toResp() {
    switch(this) {
      case Cmd.bindReceiver:
        return Cmd.bindReceiverResp;
      case Cmd.bindTransceiver:
        return Cmd.bindTransceiverResp;
      case Cmd.bindTransmitter:
        return Cmd.bindTransmitterResp;
      case Cmd.submitSm:
        return Cmd.submitSmResp;
      case Cmd.deliverSm:
        return Cmd.deliverSmResp;
      default:
        throw Exception("Cannot find resp for $this");
    }
  }

  static Cmd fromCmdId(int cmdId) {
    for (var cmd in Cmd.values) {
      if (cmdId == cmd.cmdId) {
        return cmd;
      }
    }
    return Cmd.unknown;
  }
}

enum Sts {
  ok(0x00000000),
  invalidCmd(0x00000003),
  invalidBindSts(0x00000004),
  alreadyBound(0x00000005),
  systemError(0x00000008),
  unknown(-1);

  final int stsId;

  const Sts(this.stsId);

  static Sts fromStsId(int stsId) {
    for (var sts in Sts.values) {
      if (stsId == sts.stsId) {
        return sts;
      }
    }
    return Sts.unknown;
  }
}

enum Tag {
  privacyIndicator(0x0201),
  msValidity(0x1204),
  scInterfaceVersion(0x0210),
  receiptedMessageId(0x001e),
  messageState(0x0427);

  final int _tagCode;

  const Tag(this._tagCode);
}

abstract class Pdu {

  final Cmd cmd;
  final Sts sts;
  final int seq;

  Pdu(this.cmd, this.sts, this.seq);

  void parseBody(ByteData bytes);

  ByteData toBytes();
}

class HeaderPdu extends Pdu {

  HeaderPdu(Cmd cmd, Sts sts, int seq): super(cmd, sts, seq);

  void parseBody(ByteData bytes) {
    // NOOP
  }

  ByteData toBytes() {
    var bytes = ByteData(16);
    bytes.setUint32(0, 16);
    bytes.setUint32(4, cmd.cmdId);
    bytes.setUint32(8, sts.stsId);
    bytes.setUint32(12, seq);
    return bytes;
  }
}

class BindReqPdu extends Pdu {

  String systemId = "";
  // don't interested in other fields

  BindReqPdu(Cmd cmd, Sts sts, int seq): super(cmd, sts, seq);

  @override
  void parseBody(ByteData bytes) {
    String systemId;
    (systemId, _) = bytes.getCString(16);
    this.systemId = systemId;
    // skip parsing other fields
  }

  @override
  ByteData toBytes() {
    throw Exception("unnecessary operation not implemented in smsc");
  }

  BindRespPdu createResp(Sts sts, String? systemId) {
    var resp = BindRespPdu(cmd.toResp(), sts, seq);
    resp.systemId = systemId;
    return resp;
  }
}

class BindRespPdu extends Pdu {

  String? systemId;

  BindRespPdu(Cmd cmd, Sts sts, int seq): super(cmd, sts, seq);

  @override
  void parseBody(ByteData bytes) {
    throw Exception("unnecessary operation not implemented in smsc");
  }

  @override
  ByteData toBytes() {
    var interfaceVersion = Tlv.byte(Tag.scInterfaceVersion, 0x34);
    var len = 16 + interfaceVersion._lenInBytes();
    if (Sts.ok == sts) {
      // also need space for systemId
      len += (systemId != null ? (systemId!.length + 1) : 0);
    }
    int offset = 0;
    var bytes = ByteData(len);
    bytes.setUint32(offset + 0, len);
    bytes.setUint32(offset + 4, cmd.cmdId);
    bytes.setUint32(offset + 8, sts.stsId);
    bytes.setUint32(offset + 12, seq);
    offset = 16;
    if (Sts.ok == sts && systemId != null) {
      offset = bytes.setCString(offset, systemId);
    }
    interfaceVersion._writeToBytes(bytes, offset);
    return bytes;
  }
}

class Addr {
  final String addr;
  final int ton;
  final int npi;

  Addr(this.addr, this.ton, this.npi);

  int _lenInBytes() {
    return (addr.length + 1) + 2; // +1 for null terminator
  }

  int _writeToBytes(ByteData b, int offset) {
    b.setUint8(offset++, ton);
    b.setUint8(offset++, npi);
    return b.setCString(offset, addr);
  }
}

class Tlv {
  final Tag tag;
  final int len;
  final ByteData? val;

  Tlv(Tag tag, ByteData? val):
        this.tag = tag,
        this.len = val?.lengthInBytes ?? 0,
        this.val = val;

  Tlv.byte(Tag tag, int val):
      this.tag = tag,
      this.len = 1,
      this.val = _byteVal(val);

  int _lenInBytes() {
    var len = 4;
    len += val?.lengthInBytes ?? 0;
    return len;
  }

  int _writeToBytes(ByteData b, int offset) {
    b.setUint16(offset, tag._tagCode);
    offset += 2;
    b.setUint16(offset, len);
    offset += 2;
    return b.setBytes(offset, val);
  }

  static ByteData _byteVal(int n) {
    var b = new ByteData(1);
    b.setUint8(0, n);
    return b;
  }
}

class SubmitSmPdu extends Pdu {

  Addr? srcAddr;
  Addr? destAddr;
  int regDeliv = 0;
  // don't interested in other fields

  SubmitSmPdu(Sts sts, int seq): super(Cmd.submitSm, sts, seq);

  @override
  void parseBody(ByteData bytes) {
    int offset = 16;
    (_, offset) = bytes.getCString(offset); // service_type
    int srcTon = bytes.getUint8(offset);
    int srcNpi = bytes.getUint8(offset+1);
    String src;
    (src, offset) = bytes.getCString(offset + 2);
    int destTon = bytes.getUint8(offset);
    int destNpi = bytes.getUint8(offset+1);
    String dst;
    (dst, offset) = bytes.getCString(offset + 2);
    (_, offset) = bytes.getCString(offset + 3); // sched_time (skipped esm_class, protocol_id & priority class)
    (_, offset) = bytes.getCString(offset); // validity_period
    this.regDeliv = bytes.getUint8(offset);
    this.srcAddr = Addr(src, srcTon, srcNpi);
    this.destAddr = Addr(dst, destTon, destNpi);
  }

  @override
  ByteData toBytes() {
    throw Exception("unnecessary operation not implemented in smsc");
  }

  SubmitSmRespPdu createResp(Sts sts, String? messageId) {
    var resp = SubmitSmRespPdu(sts, seq);
    resp.messageId = messageId;
    return resp;
  }
}

class SubmitSmRespPdu extends Pdu {

  String? messageId;

  SubmitSmRespPdu(Sts sts, int seq): super(Cmd.submitSmResp, sts, seq);

  @override
  void parseBody(ByteData bytes) {
    throw Exception("unnecessary operation not implemented in smsc");
  }

  @override
  ByteData toBytes() {
    var len = 16;
    if (Sts.ok == sts) {
      // also need space for message_id
      len = len + (messageId != null ? (messageId!.length + 1) : 0);
    }
    var bytes = ByteData(len);
    bytes.setUint32(0, len);
    bytes.setUint32(4, cmd.cmdId);
    bytes.setUint32(8, sts.stsId);
    bytes.setUint32(12, seq);
    if (Sts.ok == sts && messageId != null) {
      int offset = 16;
      bytes.setCString(offset, messageId);
    }
    return bytes;
  }
}

class DeliverSmPdu extends Pdu {

  String serviceType = "";
  Addr? srcAddr;
  Addr? destAddr;
  int esmClass = 0;
  int dataCoding = 0;
  ByteData? message;
  List<Tlv> optionalParams = List.empty(growable: true);

  DeliverSmPdu(Sts sts, int seq): super(Cmd.deliverSm, sts, seq);

  @override
  void parseBody(ByteData bytes) {
    throw Exception("unnecessary operation not implemented in smsc");
  }

  @override
  ByteData toBytes() {
    var len = _calcLen();
    var b = ByteData(len);
    b.setUint32(0, len);
    b.setUint32(4, cmd.cmdId);
    b.setUint32(8, sts.stsId);
    b.setUint32(12, seq);
    int offset = 16;
    offset = b.setCString(offset, serviceType);
    if (srcAddr != null) {
      offset = srcAddr!._writeToBytes(b, offset);
    } else {
      offset += 3;
    }
    if (destAddr != null) {
      offset = destAddr!._writeToBytes(b, offset);
    } else {
      offset += 3;
    }
    b.setUint8(offset++, esmClass);
    b.setUint8(offset++, 0); // protocol id
    b.setUint8(offset++, 0); // priority flag
    b.setUint8(offset++, 0); // schedule delivery time
    b.setUint8(offset++, 0); // validity period
    b.setUint8(offset++, 0); // registered delivery
    b.setUint8(offset++, 0); // replace if present
    b.setUint8(offset++, dataCoding);
    b.setUint8(offset++, 0); // sm_default_msg_id (must be set to null)
    b.setUint8(offset++, message?.lengthInBytes ?? 0);
    offset = b.setBytes(offset, message);
    for (var tlv in optionalParams) {
      offset = tlv._writeToBytes(b, offset);
    }
    return b;
  }

  int _calcLen() {
    var len = 16;
    len += (serviceType.length + 1); // c-string
    len += srcAddr?._lenInBytes() ?? 3;
    len += destAddr?._lenInBytes() ?? 3;
    len += 10; // ten 1-byte smpp fields
    len += message?.lengthInBytes ?? 0;
    for (var tlv in optionalParams) {
      len += tlv._lenInBytes();
    }
    return len;
  }
}
