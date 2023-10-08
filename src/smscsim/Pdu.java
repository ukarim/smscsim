package smscsim;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

class Pdu {

  protected final SmppCmd cmd;
  protected final SmppSts sts;
  protected final int seq;

  Pdu(SmppCmd cmd, SmppSts sts, int seq) {
    this.cmd = cmd;
    this.sts = sts;
    this.seq = seq;
  }

  byte[] toBytes() {
    throw new UnsupportedOperationException("unnecessary operation not implemented in smsc");
  }

  void parseBody(ByteBuffer buf) {
    throw new UnsupportedOperationException("unnecessary operation not implemented in smsc");
  }
}

class HeaderPdu extends Pdu {

  HeaderPdu(SmppCmd cmd, SmppSts sts, int seq) {
    super(cmd, sts, seq);
  }

  HeaderPdu(SmppCmd cmd, int seq) {
    this(cmd, SmppSts.OK, seq);
  }

  @Override
  byte[] toBytes() {
    var buf = ByteBuffer.allocate(16);
    buf.putInt(16);
    buf.putInt(cmd.getCmdId());
    buf.putInt(sts.getStsId());
    buf.putInt(seq);
    return buf.array();
  }

  @Override
  void parseBody(ByteBuffer buf) {
    // noop
  }
}

class BindReqPdu extends Pdu {

  private String systemId = "";

  BindReqPdu(SmppCmd cmd, SmppSts sts, int seq) {
    super(cmd, sts, seq);
  }

  @Override
  void parseBody(ByteBuffer buf) {
    systemId = Utils.parseCStr(buf);
  }

  String systemId() {
    return systemId;
  }

  Pdu createBindResp(SmppSts sts, int seq, String systemId) {
    int respCmdId = cmd.getCmdId() + 0x80000000; // a little hack to calc resp cmd id
    var bindResp = new BindRespPdu(SmppCmd.fromCmdId(respCmdId), sts, seq);
    if (sts == SmppSts.OK) {
      bindResp.systemId(systemId);
      bindResp.addOptionalParam(new Tlv(Tag.SC_INTERFACE_VERSION, new byte[] {0x34}));
    }
    return bindResp;
  }
}

class BindRespPdu extends Pdu {

  private String systemId = "";
  private List<Tlv> optionalParams;

  BindRespPdu(SmppCmd cmd, SmppSts sts, int seq) {
    super(cmd, sts, seq);
  }

  @Override
  byte[] toBytes() {
    byte[] systemIdBytes = systemId.getBytes(StandardCharsets.US_ASCII);
    int len = 16;
    len += (systemIdBytes.length + 1); // plus null terminator
    if (optionalParams != null) {
      for (Tlv p : optionalParams) {
        len += p.lenInBytes();
      }
    }
    var buf = ByteBuffer.allocate(len);
    buf.putInt(len);
    buf.putInt(cmd.getCmdId());
    buf.putInt(sts.getStsId());
    buf.putInt(seq);
    buf.put(systemIdBytes);
    buf.put((byte) 0); // null terminator
    if (optionalParams != null) {
      for (Tlv p : optionalParams) {
        p.writeToBuf(buf);
      }
    }
    return buf.array();
  }

  void addOptionalParam(Tlv tlv) {
    if (optionalParams == null) {
      optionalParams = new ArrayList<>(1);
    }
    optionalParams.add(tlv);
  }

  void systemId(String systemId) {
    if (systemId != null) {
      this.systemId = systemId;
    }
  }
}

class SubmitSm extends Pdu {

  private Addr srcAddr;
  private Addr destAddr;
  private int regDeliv = 0;
  // don't interested in other fields

  SubmitSm(SmppSts sts, int seq) {
    super(SmppCmd.SUBMIT_SM_RESP, sts, seq);
  }

  @Override
  void parseBody(ByteBuffer buf) {
    String serviceType = Utils.parseCStr(buf);
    srcAddr = new Addr(buf.get(), buf.get(), Utils.parseCStr(buf));
    destAddr = new Addr(buf.get(), buf.get(), Utils.parseCStr(buf));
    byte esmClass = buf.get();
    byte protocolId = buf.get();
    byte priority = buf.get();
    String sched = Utils.parseCStr(buf);
    String valid = Utils.parseCStr(buf);
    regDeliv = buf.get();

    // stop parsing (don't interested in other fields)
  }

  Addr srcAddr() {
    return srcAddr;
  }

  Addr destAddr() {
    return destAddr;
  }

  boolean isDlrRequested() {
    return regDeliv != 0;
  }

  SubmitSmResp createResp(SmppSts respSts, String messageId) {
    return new SubmitSmResp(respSts, seq, messageId);
  }
}

class SubmitSmResp extends Pdu {

  private final String messageId;

  SubmitSmResp(SmppSts sts, int seq, String messageId) {
    super(SmppCmd.SUBMIT_SM_RESP, sts, seq);
    this.messageId = messageId;
  }

  @Override
  byte[] toBytes() {
    int len = 16;
    byte[] msgIdBytes = null;
    if (SmppSts.OK == sts && messageId != null) {
      msgIdBytes = messageId.getBytes(StandardCharsets.US_ASCII);
      len += (msgIdBytes.length + 1); // plus 1 for null terminator
    }
    var buf = ByteBuffer.allocate(len);
    buf.putInt(len);
    buf.putInt(cmd.getCmdId());
    buf.putInt(sts.getStsId());
    buf.putInt(seq);
    if (SmppSts.OK == sts && msgIdBytes != null) {
      buf.put(msgIdBytes);
      buf.put((byte) 0); // null terminator
    }
    return buf.array();
  }
}

class DeliverSm extends Pdu {

  private byte[] serviceType = new byte[0];
  private Addr srcAddr;
  private Addr destAddr;
  private byte esmClass;
  private byte dataCoding;
  private byte[] message = new byte[0];
  private List<Tlv> optionalParams = Collections.emptyList();

  DeliverSm(SmppSts sts, int seq) {
    super(SmppCmd.DELIVER_SM, sts, seq);
  }

  void serviceType(String serviceType) {
    this.serviceType = serviceType.getBytes(StandardCharsets.US_ASCII);
  }

  void srcAddr(Addr srcAddr) {
    this.srcAddr = srcAddr;
  }

  void destAddr(Addr destAddr) {
    this.destAddr = destAddr;
  }

  void esmClass(byte esmClass) {
    this.esmClass = esmClass;
  }

  void dataCoding(byte dataCoding) {
    this.dataCoding = dataCoding;
  }

  void message(byte[] message) {
    this.message = message;
  }

  void addOptionalParam(Tlv tlv) {
    if (optionalParams.isEmpty()) {
      optionalParams = new ArrayList<>(1);
    }
    optionalParams.add(tlv);
  }

  @Override
  byte[] toBytes() {
    int len = calcLen();
    var buf = ByteBuffer.allocate(len);
    buf.putInt(len);
    buf.putInt(cmd.getCmdId());
    buf.putInt(sts.getStsId());
    buf.putInt(seq);
    buf.put(serviceType).put((byte) 0); // with null-terminator
    srcAddr.writeToBuf(buf);
    destAddr.writeToBuf(buf);
    buf.put(esmClass);
    buf.put((byte) 0); // protocol id
    buf.put((byte) 0); // priority flag
    buf.put((byte) 0); // schedule delivery time
    buf.put((byte) 0); // validity period
    buf.put((byte) 0); // registered delivery
    buf.put((byte) 0); // replace if present
    buf.put(dataCoding);
    buf.put((byte) 0); // sm_default_msg_id (must be set to null)
    buf.put((byte) message.length);
    buf.put(message);
    for (Tlv tlv : optionalParams) {
      tlv.writeToBuf(buf);
    }
    return buf.array();
  }

  private int calcLen() {
    int len = 16;
    len += (serviceType.length + 1);
    len += srcAddr.lenInBytes();
    len += destAddr.lenInBytes();
    len += 10; // 10 one-byte fields;
    len += message.length;
    for(Tlv tlv : optionalParams) {
      len += tlv.lenInBytes();
    }
    return len;
  }
}

enum SmppCmd {

  GENERIC_NACK(0x80000000),
  BIND_RECEIVER(0x00000001),
  BIND_RECEIVER_RESP(0x80000001),
  BIND_TRANSMITTER(0x00000002),
  BIND_TRANSMITTER_RESP(0x80000002),
  SUBMIT_SM(0x00000004),
  SUBMIT_SM_RESP(0x80000004),
  DELIVER_SM(0x00000005),
  DELIVER_SM_RESP(0x80000005),
  UNBIND(0x00000006),
  UNBIND_RESP(0x80000006),
  BIND_TRANSCEIVER(0x00000009),
  BIND_TRANSCEIVER_RESP(0x80000009),
  ENQUIRE_LINK(0x00000015),
  ENQUIRE_LINK_RESP(0x80000015),
  ;

  private final int cmdId;

  SmppCmd(int cmdId) {
    this.cmdId = cmdId;
  }

  public int getCmdId() {
    return cmdId;
  }

  static SmppCmd fromCmdId(int cmdId) {
    SmppCmd[] vals = SmppCmd.values();
    for (SmppCmd cmd : vals) {
      if (cmd.cmdId == cmdId) {
        return cmd;
      }
    }
    throw new RuntimeException("Unknown smpp command with id " + cmdId);
  }
}

enum SmppSts {
  OK(0x00000000),
  INV_CMD_ID(0x00000003),
  INV_BIND_STS(0x00000004),
  ALREADY_BOUND(0x00000005),
  SYS_ERROR(0x00000008),
  UNKNOWN(0x000000FF),
  ;

  private final int stsId;

  SmppSts(int stsId) {
    this.stsId = stsId;
  }

  int getStsId() {
    return stsId;
  }

  static SmppSts fromStsId(int stsId) {
    for (SmppSts sts : SmppSts.values()) {
      if (sts.stsId == stsId) {
        return sts;
      }
    }
    throw new RuntimeException("Unknown smpp status with id " + stsId);
  }
}

enum Tag {
  PRIVACY_INDICATOR((short) 0x0201),
  MS_VALIDITY((short) 0x1204),
  SC_INTERFACE_VERSION((short) 0x0210),
  RECEIPTED_MSG_ID((short) 0x001e),
  MSG_STATE((short) 0x0427),
  ;

  private final short tagId;

  Tag(short tagId) {
    this.tagId = tagId;
  }

  short getTagId() {
    return tagId;
  }
}

class Tlv {
  private final Tag tag;
  private final short len;
  private final byte[] val;

  Tlv(Tag tag, byte[] val) {
    this.tag = tag;
    this.len = (short) val.length;
    this.val = val;
  }

  int lenInBytes() {
    return 4 + val.length;
  }

  void writeToBuf(ByteBuffer buf) {
    buf.putShort(tag.getTagId());
    buf.putShort(len);
    buf.put(val);
  }
}

record Addr(byte ton, byte npi, String addr) {

  int lenInBytes() {
    int len = 3; // npi + ton + null terminator
    if (addr != null) {
      len += addr.getBytes(StandardCharsets.US_ASCII).length;
    }
    return len;
  }

  void writeToBuf(ByteBuffer buf) {
    buf.put(ton);
    buf.put(npi);
    if (addr != null) {
      buf.put(addr.getBytes(StandardCharsets.US_ASCII));
    }
    buf.put((byte) 0);
  }

  @Override
  public String toString() {
    return ton + ":" + npi + ":" + addr;
  }
}
