import 'dart:typed_data';

import 'pdu.dart';
import 'util.dart';

void main() {
  testHeaderPdu();
  testBindRespPdu();
  testSubmitSmRespPdu();
  testDeliverSmPdu();
}

void testHeaderPdu() {
  var expectedBytes = Uint8List.fromList([
    0x00, 0x00, 0x00, 0x10,
    0x00, 0x00, 0x00, 0x15,
    0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0xEA,
  ]);
  var enqLink = HeaderPdu(Cmd.enquireLink, Sts.ok, 234);
  assertByteDataEquals(
      expectedBytes.buffer.asByteData(),
      enqLink.toBytes(),
      "PDU header incorrectly encoded"
  );
}

void testBindRespPdu() {
  var expectedBytes = Uint8List.fromList([
    0x00, 0x00, 0x00, 0x1C,
    0x80, 0x00, 0x00, 0x02,
    0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x84,
    0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x00,
    0x02, 0x10, 0x00, 0x01, 0x34
  ]);
  var bindResp = BindRespPdu(Cmd.bindTransmitterResp, Sts.ok, 132);
  bindResp.systemId = "123456";
  assertByteDataEquals(
      expectedBytes.buffer.asByteData(),
      bindResp.toBytes(),
      "BindRespPdu incorrectly encoded"
  );
}

void testSubmitSmRespPdu() {
  var expectedBytes = Uint8List.fromList([
    0x00, 0x00, 0x00, 0x17,
    0x80, 0x00, 0x00, 0x04,
    0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x84,
    0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x00
  ]);
  var submitSmResp = SubmitSmRespPdu(Sts.ok, 132);
  submitSmResp.messageId= "123456";
  assertByteDataEquals(
      expectedBytes.buffer.asByteData(),
      submitSmResp.toBytes(),
      "SubmitSmResp incorrectly encoded"
  );
}

void testDeliverSmPdu() {
  var expectedBytes = Uint8List.fromList([
    0x00, 0x00, 0x00, 0x45, // command_length
    0x00, 0x00, 0x00, 0x05, // command_id
    0x00, 0x00, 0x00, 0x00, // command_status
    0x00, 0x00, 0x00, 0x66, // sequence_number
    0x73, 0x6d, 0x73, 0x63, 0x73, 0x69, 0x6d, 0x00, // service_type
    0x00, 0x00, 0x37, 0x37, 0x30, 0x31, 0x32, 0x31, 0x31, 0x30, 0x30, 0x30, 0x30, 0x00, // source_addr_ton, source_addr_npi, source_addr
    0x00, 0x00, 0x31, 0x30, 0x30, 0x31, 0x00, // dest_addr_ton, dest_addr_npi, destination_addr
    0x00, // esm class
    0x00, // protocol_id
    0x00, // priority_flag
    0x00, // schedule_delivery_time
    0x00, // validity_period
    0x00, // registered_delivery
    0x00, // replace_if_present_flag
    0x00, // data_coding
    0x00, // sm_default_msg_id
    0x04, // sm_length
    0x54, 0x65, 0x73, 0x74, // short_message
    0x02, 0x01, 0x00, 0x01, 0x03, // privacy_indicator tlv
    0x12, 0x04, 0x00, 0x01, 0x02, // ms_validity tlv
  ]);
  var deliverSm = DeliverSmPdu(Sts.ok, 102);
  deliverSm.serviceType = "smscsim";
  deliverSm.srcAddr = Addr("77012110000", 0, 0);
  deliverSm.destAddr = Addr("1001", 0, 0);
  deliverSm.message = "Test".asBytes();
  deliverSm.dataCoding = 0x00;
  deliverSm.optionalParams.add(Tlv.byte(Tag.privacyIndicator, 3));
  deliverSm.optionalParams.add(Tlv.byte(Tag.msValidity, 2));
  assertByteDataEquals(
      expectedBytes.buffer.asByteData(),
      deliverSm.toBytes(),
      "DeliverSm incorrectly encoded"
  );
}

void assertByteDataEquals(ByteData expected, ByteData actual, String message) {
  if (expected.lengthInBytes != actual.lengthInBytes) {
    print("${Uint8List.view(expected.buffer)}\n${Uint8List.view(actual.buffer)}");
    throw Exception(message);
  }
  for (int i = 0; i < expected.lengthInBytes; i++) {
    if (expected.getUint8(i) != actual.getUint8(i)) {
      print("${Uint8List.view(expected.buffer)}\n${Uint8List.view(actual.buffer)}");
      throw Exception("$message\nSee byte at position $i");
    }
  }
}
