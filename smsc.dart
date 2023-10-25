import 'dart:async';
import 'dart:collection';
import 'dart:io';
import 'dart:math';
import 'dart:typed_data';

import 'pdu.dart';
import 'util.dart';

class _Session {
  int? sessionId;
  String? systemId;
  Socket? socket;
  bool receiveMo = false;
  bool bound = false;
  bool receiver = false;
  int seqNumCounter = 1;

  void bind(int sessionId, String systemId, Cmd cmd, Socket socket) {
    this.sessionId = sessionId;
    this.systemId = systemId;
    this.receiver = cmd == Cmd.bindReceiver;
    this.receiveMo = (cmd == Cmd.bindReceiver || cmd == Cmd.bindTransceiver);
    this.bound = true;
    this.socket = socket;
  }

  void unbind() {
    sessionId = null;
    systemId = null;
    socket = null;
    receiveMo = false;
    bound = false;
    receiver = false;
    seqNumCounter = 1;
  }

  int nextSeqNum() => seqNumCounter++;
}

class SmscServer {

  final bool _failedSubmits;
  final Map<int, _Session> _smppSessions = LinkedHashMap();
  int _sessionIdCounter = 1;
  int _messageIdCounter = 1;

  SmscServer(this._failedSubmits);

  void start(int port) async {
    logInfo("Starting SMSc simulator on port $port");
    ServerSocket serverSocket = await ServerSocket.bind(InternetAddress.anyIPv4, port);
    serverSocket.listen(_handleConnection);
  }

  List<String> boundSystemIds() {
    return List.of(_smppSessions.values.map((e) => e.systemId!), growable: false);
  }

  (String, bool) sendMoMessage(String sender, String recipient, String message, String systemId) {
    logInfo("sending MO, sender=$sender, recipient=$recipient, message=$message, systemId=$systemId");
    _Session? session;
    for (var s in _smppSessions.values) {
      if (s.systemId == systemId) {
        session = s;
        break;
      }
    }
    if (session == null) {
      logError("Cannot send MO message to systemId: $systemId. No bound session found");
      return ("No session found for systemId: $systemId", true);
    }
    if (!session.receiveMo) {
      logError("Cannot send MO message to systemId: $systemId. Only RECEIVER and TRANSCEIVER sessions could receive MO messages");
      return ("Only RECEIVER and TRANSCEIVER sessions could receive MO messages", true);
    }
    // TODO implement UDH for large messages
    var shortMsg = _truncate(message, 70);
    var deliverSmPdu = DeliverSmPdu(Sts.ok, session.nextSeqNum());
    deliverSmPdu.serviceType = "smscsim";
    deliverSmPdu.srcAddr = Addr(sender, 0, 0);
    deliverSmPdu.destAddr = Addr(recipient, 0, 0);
    deliverSmPdu.dataCoding = 0x08; // UCS2
    deliverSmPdu.message = shortMsg.asUCS2Bytes();
    session.socket!.add(Uint8List.view(deliverSmPdu.toBytes().buffer));
    return ("OK", false);
  }

  void _handleConnection(Socket socket) {
    var session = _Session();
    var dataHandler = _DataHandler(this, session, socket);
    socket.listen(dataHandler, onError: (error) {
        logError("Unexpected error $e. Closing connection for ${session.systemId}");
        socket.destroy();
      }, onDone: () {
        if (session.bound) {
          logError("Connection for ${session.systemId} unexpectedly closed");
        }
        socket.destroy();
      });
  }

  String _truncate(String s, int lenInBytes) {
    var buf = StringBuffer();
    int c = 0;
    int max = lenInBytes * 8;
    for (var r in s.runes) {
      c += r.bitLength;
      if (c > max) {
        break;
      }
      buf.writeCharCode(r);
    }
    return buf.toString();
  }
}

class _DataHandler {

  final SmscServer smscServer;
  final _Session session;
  final Socket socket;

  _DataHandler(this.smscServer, this.session, this.socket);

  void call(Uint8List byteList) {
    try {
      // TODO handle partial data
      internalCall(byteList);
    } catch (e) {
      logError("Unexpected error: $e. Closing connection");
      socket.destroy();
    }
  }

  void internalCall(Uint8List byteList) {
    final byteBuffer = byteList.buffer;
    final byteData = byteBuffer.asByteData();
    int offset = 0;
    // var _len = byteData.getUint32(offset);
    var cmd = Cmd.fromCmdId(byteData.getUint32(offset + 4));
    var sts = Sts.fromStsId(byteData.getUint32(offset + 8));
    var seqNum = byteData.getUint32(offset + 12);
    offset = 16;

    String? boundSystemId = session.systemId;
    Pdu? respPdu;

    switch(cmd) {
      case Cmd.bindReceiver || Cmd.bindTransmitter || Cmd.bindTransceiver: {
        var bindReqPdu = BindReqPdu(cmd, sts, seqNum);
        bindReqPdu.parseBody(byteData);
        var systemId = bindReqPdu.systemId;
        logInfo("bind request from $systemId");
        Sts respSts;
        if (session.bound) {
          respSts = Sts.alreadyBound;
          logError("$systemId already in bound state");
        } else {
          var sessionId = smscServer._sessionIdCounter++;
          session.bind(sessionId, systemId, cmd, socket);
          smscServer._smppSessions[sessionId] = session;
          respSts = Sts.ok;
        }
        respPdu = bindReqPdu.createResp(respSts, "smscsim");
      }
      case Cmd.unbind: {
        logInfo("unbind request from $boundSystemId");
        smscServer._smppSessions.remove(session.sessionId);
        session.unbind();
        respPdu = HeaderPdu(Cmd.unbindResp, Sts.ok, seqNum);
      }
      case Cmd.enquireLink: {
        // just respond to heartbeat req
        respPdu = HeaderPdu(Cmd.enquireLinkResp, Sts.ok, seqNum);
      }
      case Cmd.submitSm: {
        if (session.receiver) {
          logError("Error while handling submit_sm from $boundSystemId. Session with RECEIVER type cannot send requests");
          respPdu = HeaderPdu(Cmd.submitSmResp, Sts.invalidBindSts, seqNum);
          break;
        }
        var submitSmPdu = SubmitSmPdu(sts, seqNum);
        submitSmPdu.parseBody(byteData);
        var src = submitSmPdu.srcAddr?.addr;
        var dst = submitSmPdu.destAddr?.addr;

        logInfo("Received submit_sm from $boundSystemId. Src:$src, dest:$dst");

        String? msgId = null;
        Sts respSts;
        if (smscServer._failedSubmits && seqNum % 2 == 0) {
          respSts = Sts.systemError;
        } else {
          msgId = "${smscServer._messageIdCounter++}";
          respSts = Sts.ok;
          if (submitSmPdu.regDeliv != 0) {
            var sbmDate = DateTime.now();
            Future.delayed(const Duration(seconds: 2), () {
              var dlrReceipt = _createDlrReceipt(msgId!, sbmDate, DateTime.now(), smscServer._failedSubmits);
              var deliverSm = DeliverSmPdu(Sts.ok, session.nextSeqNum());
              deliverSm.srcAddr = submitSmPdu.destAddr;
              deliverSm.destAddr = submitSmPdu.srcAddr;
              deliverSm.esmClass = 4; // for DLR
              deliverSm.message = dlrReceipt.asBytes();
              int msgStateVal = smscServer._failedSubmits ? 5 : 2;
              deliverSm.optionalParams.add(Tlv.byte(Tag.messageState, msgStateVal));
              ByteData receiptedMsgIdVal = msgId.asCStringBytes();
              deliverSm.optionalParams.add(Tlv(Tag.receiptedMessageId, receiptedMsgIdVal));

              socket.add(Uint8List.view(deliverSm.toBytes().buffer));
            });
          }
        }
        respPdu = submitSmPdu.createResp(respSts, msgId);
      }
      case Cmd.deliverSmResp: {
        // Do nothing
        logInfo("Received deliver_sm_resp from $boundSystemId");
      }
      default: {
        logError("Unsupported pdu cmd($cmd) from $boundSystemId. Sending generic_nack");
        respPdu = HeaderPdu(Cmd.genericNack, Sts.ok, seqNum);
      }
    }

    if (respPdu != null) {
      socket.add(Uint8List.view(respPdu.toBytes().buffer));
    }
  }

  String _createDlrReceipt(String msgId, DateTime sbmDate, DateTime doneDate, bool failed) {
    var dlvrd = failed ? "0" : "1";
    var stat = failed ? "UNDELIV" : "DELIVRD";
    var err = failed ? "069" : "000";
    var sbmDateFmt = _formatDlrTime(sbmDate);
    var doneDateFmt = _formatDlrTime(doneDate);
    return "id:$msgId sub:001 dlvrd:00$dlvrd submit date:$sbmDateFmt done date:$doneDateFmt stat:$stat err:$err Text:...";
  }

  String _formatDlrTime(DateTime dt) {
    var y = "${dt.year}".substring(2); // need only two last digits
    var M = "${dt.month}".padLeft(2, "0");
    var d = "${dt.day}".padLeft(2, "0");
    var h = "${dt.hour}".padLeft(2, "0");
    var m = "${dt.minute}".padLeft(2, "0");
    return "$y$M$d$h$m";
  }
}