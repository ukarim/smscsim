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
	actualBytes := pduHeaderBytes(ENQUIRE_LINK, STS_OK, 234)
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
	actualBytes := pduWithStringBodyBytes(DELIVER_SM_RESP, STS_OK, 132, "123456")
	if !reflect.DeepEqual(expectedBytes, actualBytes) {
		fmt.Printf("expected: [%s]\nactual: [%s]\n\n", hex.EncodeToString(expectedBytes), hex.EncodeToString(actualBytes))
		t.Errorf("PDU with string body incorrectly encoded")
	}
}
