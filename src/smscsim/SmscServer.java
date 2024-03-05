package smscsim;

import java.io.OutputStream;
import java.net.ServerSocket;
import java.net.Socket;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.Collectors;

import static java.lang.System.Logger.Level.ERROR;
import static java.lang.System.Logger.Level.INFO;

class SmscServer implements Runnable {

  private static final DateTimeFormatter DLR_DATE_FORMATTER = DateTimeFormatter.ofPattern("yyMMddHHmm");

  private final System.Logger logger = System.getLogger("SmscServer");
  private final int port;
  private final boolean failedSubmits;

  private final Map<Integer, Session> boundSessions = new ConcurrentHashMap<>();
  private final AtomicLong messageIdCounter = new AtomicLong(1_000_000);

  SmscServer(int port, boolean failedSubmits) {
    this.port = port;
    this.failedSubmits = failedSubmits;
  }

  public void run() {
    logger.log(INFO, "Starting smpp server on port " + port);
    try (var serverSocket = new ServerSocket(port, 0)) {
      while (true) {
        Socket clientSocket = serverSocket.accept();
        Thread.startVirtualThread(() -> handleClientSocket(clientSocket));
      }
    } catch (Exception e) {
      logger.log(ERROR, "Error starting SmscServer", e);
      System.exit(1);
    }
  }

  private void handleClientSocket(Socket clientSocket) {
    var session = new Session();
    try(var socket = clientSocket;
        var in = socket.getInputStream();
        var out = socket.getOutputStream()) {

      while (true) {
        // read one pdu
        byte[] cmdLenBytes = in.readNBytes(4);
        if (cmdLenBytes.length == 0) {
          // seems like client unexpectedly closed connection
          if (session.bound) {
            logger.log(ERROR, "connection has been closed by " + session);
          }
          return;
        }
        int cmdLen = Utils.parseInt(cmdLenBytes, 0);
        var pduBytes = ByteBuffer.wrap(in.readNBytes(cmdLen - 4));

        // parse pdu header
        var cmd = SmppCmd.fromCmdId(pduBytes.getInt());
        SmppSts sts = SmppSts.fromStsId(pduBytes.getInt());
        int seq = pduBytes.getInt();

        Pdu resp = switch (cmd) {
          case ENQUIRE_LINK -> new HeaderPdu(SmppCmd.ENQUIRE_LINK_RESP, seq); // respond heartbeat
          case DELIVER_SM_RESP -> {
            logger.log(INFO, "receiver deliver_sm_resp from " + session);
            // we received response (just ignore it)
            yield null;
          }
          case UNBIND -> {
            logger.log(INFO, "unbind request from " + session);
            boundSessions.remove(session.sessionId);
            session.unbind();
            yield new HeaderPdu(SmppCmd.UNBIND_RESP, seq);
          }
          case BIND_RECEIVER, BIND_TRANSCEIVER, BIND_TRANSMITTER -> {
            var bindReq = new BindReqPdu(cmd, sts, seq);
            bindReq.parseBody(pduBytes);
            String systemId = bindReq.systemId();
            logger.log(INFO, "Bind request from " + systemId);
            SmppSts respSts;
            if (session.bound) {
              logger.log(ERROR, systemId + " already in bound state");
              respSts = SmppSts.ALREADY_BOUND;
            } else {
              session.bind(systemId, cmd, socket);
              boundSessions.put(session.sessionId, session);
              respSts = SmppSts.OK;
            }
            yield bindReq.createBindResp(respSts, seq, "smscsim");
          }
          case SUBMIT_SM -> {
            if (session.receiver) {
              // receiver cannot handle submit_sm packet
              logger.log(ERROR, "error while handling submit_sm from "+ session +". Session with RECEIVER type cannot send requests");
              yield new HeaderPdu(SmppCmd.SUBMIT_SM_RESP, SmppSts.INV_BIND_STS, seq);
            }
            var submitSm = new SubmitSm(sts, seq);
            submitSm.parseBody(pduBytes);
            logger.log(INFO, String.format("received submit_sm from %s. src: %s, dest: %s", session, submitSm.srcAddr(), submitSm.destAddr()));

            SmppSts respSts;
            String msgId = null;

            if (failedSubmits && seq % 2 == 0) {
              respSts = SmppSts.SYS_ERROR;
            } else {
              msgId = "" + messageIdCounter.incrementAndGet();
              respSts = SmppSts.OK;
              if (submitSm.isDlrRequested()) {
                sendDlrWithDelay(msgId, submitSm, session);
              }
            }
            yield submitSm.createResp(respSts, msgId);
          }
          default -> {
            logger.log(ERROR, "unsupported pdu cmd(" + cmd.getCmdId() + ") from " + session);
            yield new HeaderPdu(SmppCmd.GENERIC_NACK, SmppSts.INV_CMD_ID, seq);
          }
        };

        if (resp != null) {
          // send response
          out.write(resp.toBytes());
          logger.log(INFO, resp.cmd + " pdu was sent to " + session);
        }
      }
    } catch (Exception e) {
      logger.log(ERROR, "Error handling client connection", e);
    } finally {
      boundSessions.remove(session.sessionId);
    }
  }

  private void sendDlrWithDelay(String msgId, SubmitSm submitSm, Session session) {
    Thread.startVirtualThread(() -> {
      try {
        LocalDateTime sbmDate = LocalDateTime.now();
        Thread.sleep(2_000);
        String dlrReceipt = createDlrReceipt(msgId, sbmDate, LocalDateTime.now(), failedSubmits);
        var deliverSm = new DeliverSm(SmppSts.OK, session.nextSeqNum());
        deliverSm.srcAddr(submitSm.destAddr());
        deliverSm.destAddr(submitSm.srcAddr());
        deliverSm.esmClass((byte) 4); // for DLR
        deliverSm.message(dlrReceipt.getBytes(StandardCharsets.US_ASCII));
        int msgState = failedSubmits ? 5 : 2;
        deliverSm.addOptionalParam(new Tlv(Tag.MSG_STATE, new byte[] {(byte) msgState}));
        byte[] msgIdBytes = msgId.getBytes(StandardCharsets.US_ASCII);
        byte[] receiptedMsgId = new byte[msgIdBytes.length + 1];
        // ugh, doing that only to add null terminator
        System.arraycopy(msgIdBytes, 0, receiptedMsgId, 0, msgIdBytes.length);
        receiptedMsgId[receiptedMsgId.length - 1] = 0;
        deliverSm.addOptionalParam(new Tlv(Tag.RECEIPTED_MSG_ID, receiptedMsgId));

        OutputStream out = session.socket.getOutputStream();
        out.write(deliverSm.toBytes());
      } catch (Exception e) {
        logger.log(ERROR, "error while sending DLR to " + session, e);
      }
    });
  }

  List<String> boundSystemIds() {
      return boundSessions.values()
          .stream()
          .map(s -> s.systemId)
          .collect(Collectors.toList());
  }

  Optional<String> sendMoMessage(String sender, String recipient, String message, String systemId) {
    logger.log(INFO, String.format("Got request to send MO message: %s, %s, %s, %s", sender, recipient, message, systemId));
    Session session = null;
    for (Session s : boundSessions.values()) {
      if (s.systemId.equals(systemId)) {
        session = s;
        break;
      }
    }
    if (session == null) {
      logger.log(ERROR, "Cannot send MO message to systemId: " + systemId + ". No bound session found");
      return Optional.of("No session found for systemId: " + systemId);
    }
    if (!session.receiveMo) {
      logger.log(ERROR, "Cannot send MO message to systemId: " + systemId + ". Only RECEIVER and TRANSCEIVER sessions could receive MO messages");
      return Optional.of("Only RECEIVER and TRANSCEIVER sessions could receive MO messages");
    }
    List<DeliverSm> pdus = new ArrayList<>();
    List<byte[]> udhParts = Utils.toUdhParts(message.getBytes(StandardCharsets.UTF_16BE));
    byte esmClass = (byte) (udhParts.size() > 1 ? 0x40 : 0x00);
    for (byte[] sm : udhParts) {
      var deliverSm = new DeliverSm(SmppSts.OK, session.nextSeqNum());
      deliverSm.serviceType("smscsim");
      deliverSm.srcAddr(new Addr((byte) 0, (byte) 0, sender));
      deliverSm.destAddr(new Addr((byte) 0, (byte) 0, recipient));
      deliverSm.dataCoding((byte) 0x08); // UCS2
      deliverSm.esmClass(esmClass);
      deliverSm.message(sm);
      pdus.add(deliverSm);
    }
    try {
      OutputStream out = session.socket.getOutputStream();
      for (Pdu p : pdus) {
        out.write(p.toBytes());
      }
    } catch (Exception e) {
      logger.log(ERROR, "error while sending MO message to " + session, e);
      return Optional.of(e.getMessage());
    }
    return Optional.empty();
  }

  private String createDlrReceipt(String msgId, LocalDateTime sbmDate, LocalDateTime doneDate, boolean failed) {
    var dlvrd = failed ? "0" : "1";
    var stat = failed ? "UNDELIV" : "DELIVRD";
    var err = failed ? "069" : "000";
    String sbmDateFmt = sbmDate.format(DLR_DATE_FORMATTER);
    String doneDateFmt = doneDate.format(DLR_DATE_FORMATTER);
    return String.format("id:%s sub:001 dlvrd:00%s submit date:%s done date:%s stat:%s err:%s Text:...",
        msgId, dlvrd, sbmDateFmt, doneDateFmt, stat, err);
  }

  private String truncate(String s, int len) {
    if (s.length() <= len) {
      return s;
    }
    return s.substring(0, len);
  }

  private static class Session {
    private static final SecureRandom RAND = new SecureRandom();
    private final int sessionId = RAND.nextInt();
    private String systemId;
    private Socket socket;
    private boolean receiveMo;
    private boolean bound;
    private boolean receiver;
    private final AtomicInteger seqNum = new AtomicInteger();

    void bind(String systemId, SmppCmd cmd, Socket socket) {
      this.systemId = systemId;
      this.receiver = cmd == SmppCmd.BIND_RECEIVER;
      this.receiveMo = (cmd == SmppCmd.BIND_RECEIVER || cmd == SmppCmd.BIND_TRANSCEIVER);
      this.bound = true;
      this.socket = socket;
    }

    void unbind() {
      systemId = null;
      socket = null;
      receiveMo = false;
      bound = false;
      receiver = false;
      seqNum.set(0);
    }

    int nextSeqNum() {
      return seqNum.incrementAndGet();
    }

    @Override
    public String toString() {
      return "[" + systemId + "]";
    }
  }
}