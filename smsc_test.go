package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"reflect"
	"testing"
	"time"
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

func TestDeliverSmServiceTypeLength(t *testing.T) {
	pdu := deliverSmPDU("1001", "77012110000", []byte("Test"), CODING_DEFAULT, 1, 0x04, nil)
	// service_type starts right after the 16 octet header and must fit in 6 octets (SMPP 3.4, 5.2.11)
	end := bytes.IndexByte(pdu[16:], 0)
	if end < 0 || end+1 > 6 {
		t.Fatalf("service_type is %d octets, max is 6", end+1)
	}
}

func TestDeliverSmPduBytes(t *testing.T) {
	expectedBytes := []byte{
		0x00, 0x00, 0x00, 0x3e, // command_length
		0x00, 0x00, 0x00, 0x05, // command_id
		0x00, 0x00, 0x00, 0x00, // command_status
		0x00, 0x00, 0x00, 0x66, // sequence_number
		0x00,                                                                               // service_type (empty)
		0x00, 0x00, 0x37, 0x37, 0x30, 0x31, 0x32, 0x31, 0x31, 0x30, 0x30, 0x30, 0x30, 0x00, // source_addr_ton, source_addr_npi, source_addr
		0x00, 0x00, 0x31, 0x30, 0x30, 0x31, 0x00, // dest_addr_ton, dest_addr_npi, destination_addr
		0x40,                   // esm class
		0x00,                   // protocol_id
		0x00,                   // priority_flag
		0x00,                   // schedule_delivery_time
		0x00,                   // validity_period
		0x00,                   // registered_delivery
		0x00,                   // replace_if_present_flag
		0x00,                   // data_coding
		0x00,                   // sm_default_msg_id
		0x04,                   // sm_length
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

	actualBytes := deliverSmPDU(source_addr, destination_addr, []byte(short_message), 0, sequence_number, 0x40, tlv_list)
	if !reflect.DeepEqual(expectedBytes, actualBytes) {
		fmt.Printf("expected: [%s]\nactual: [%s]\n\n", hex.EncodeToString(expectedBytes), hex.EncodeToString(actualBytes))
		t.Errorf("PDU with string body incorrectly encoded")
	}
}

func TestNewSeqNumRange(t *testing.T) {
	for i := 0; i < 100000; i++ {
		if n := newSeqNum(); n < 1 || n > 0x7FFFFFFF {
			t.Fatalf("sequence_number out of range: %d", n)
		}
	}
}

func pduHeader(cmdLen, cmdId, seqNum uint32) []byte {
	h := make([]byte, 16)
	binary.BigEndian.PutUint32(h[0:], cmdLen)
	binary.BigEndian.PutUint32(h[4:], cmdId)
	binary.BigEndian.PutUint32(h[12:], seqNum)
	return h
}

func TestShortCommandLengthClosesConnection(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	done := make(chan struct{})
	go func() {
		handleSmppConnection(NewSmsc(false), server)
		close(done)
	}()
	client.Write(pduHeader(8, ENQUIRE_LINK, 1)) // command_length < 16
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("connection was not closed")
	}
}

func TestTruncatedSubmitSmGetsGenericNack(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	go handleSmppConnection(NewSmsc(false), server)
	// all fields up to validity_period, then the PDU ends (registered_delivery missing)
	body := []byte{
		0x00,                  // service_type
		0x00, 0x00, '1', 0x00, // source_addr_ton, source_addr_npi, source_addr
		0x00, 0x00, '2', 0x00, // dest_addr_ton, dest_addr_npi, destination_addr
		0x00, 0x00, 0x00, // esm_class, protocol_id, priority_flag
		0x00, // schedule_delivery_time
		0x00, // validity_period
	}
	client.Write(append(pduHeader(uint32(16+len(body)), SUBMIT_SM, 7), body...))
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp := make([]byte, 16)
	if _, err := io.ReadFull(client, resp); err != nil {
		t.Fatalf("no response: %v", err)
	}
	if id := binary.BigEndian.Uint32(resp[4:]); id != GENERIC_NACK {
		t.Fatalf("expected generic_nack, got command_id 0x%08x", id)
	}
	if seq := binary.BigEndian.Uint32(resp[12:]); seq != 7 {
		t.Fatalf("expected sequence_number 7, got %d", seq)
	}
}
