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
		0x00, 0x00, 0x00, 0x3B,
		0x00, 0x00, 0x00, 0x05,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x66,
		0x73, 0x6d, 0x73, 0x63, 0x73, 0x69, 0x6d, 0x00,
		0x00, 0x00, 0x37, 0x37, 0x30, 0x31, 0x32, 0x31, 0x31, 0x30, 0x30, 0x30, 0x30, 0x00,
		0x00, 0x00, 0x31, 0x30, 0x30, 0x31, 0x00,
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x00, 0x04,
		0x54, 0x65, 0x73, 0x74,
	}
	actualBytes := deliverSmPDU("77012110000", "1001", "Test", 102, []Tlv{})
	if !reflect.DeepEqual(expectedBytes, actualBytes) {
		fmt.Printf("expected: [%s]\nactual: [%s]\n\n", hex.EncodeToString(expectedBytes), hex.EncodeToString(actualBytes))
		t.Errorf("PDU with string body incorrectly encoded")
	}
}