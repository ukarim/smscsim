package main

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"
)

func TestPduHeaderBytes(t *testing.T) {
	expectedBytes := []byte{
		0x00, 0x00, 0x00, 0x10,
		0x00, 0x00, 0x00, 0x15,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0xEA,
	}
	actualBytes := headerPDU(ENQUIRE_LINK, STS_OK, 234)
	if !reflect.DeepEqual(expectedBytes, actualBytes) {
		fmt.Printf("expected: [%s]\nactual: [%s]\n\n", hex.EncodeToString(expectedBytes), hex.EncodeToString(actualBytes))
		t.Errorf("PDU header incorrectly encoded")
	}
}

func TestStringBodyPduBytes(t *testing.T) {
	expectedBytes := []byte{
		0x00, 0x00, 0x00, 0x17,
		0x80, 0x00, 0x00, 0x05,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x84,
		0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x00,
	}
	actualBytes := stringBodyPDU(DELIVER_SM_RESP, STS_OK, 132, "123456")
	if !reflect.DeepEqual(expectedBytes, actualBytes) {
		fmt.Printf("expected: [%s]\nactual: [%s]\n\n", hex.EncodeToString(expectedBytes), hex.EncodeToString(actualBytes))
		t.Errorf("PDU with string body incorrectly encoded")
	}
}

func TestDeliverSmPduBytes(t *testing.T) {
	expectedBytes := []byte{
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
	}

	source_addr := "77012110000"
	destination_addr := "1001"
	short_message := "Test"
	sequence_number := 102
	privacy_indicator_tlv := Tlv{0x0201, 1, []byte{3}}
	ms_validity_tlv := Tlv{0x1204, 1, []byte{2}}
	tlv_list := []Tlv{privacy_indicator_tlv, ms_validity_tlv}

	actualBytes := deliverSmPDU(source_addr, destination_addr, []byte(short_message), 0, sequence_number, tlv_list)
	if !reflect.DeepEqual(expectedBytes, actualBytes) {
		fmt.Printf("expected: [%s]\nactual: [%s]\n\n", hex.EncodeToString(expectedBytes), hex.EncodeToString(actualBytes))
		t.Errorf("PDU with string body incorrectly encoded")
	}
}
