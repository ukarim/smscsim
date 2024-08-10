package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
)

// command id
const (
	GENERIC_NACK      = 0x80000000
	BIND_RECEIVER     = 0x00000001
	BIND_TRANSMITTER  = 0x00000002
	BIND_TRANSCEIVER  = 0x00000009
	SUBMIT_SM         = 0x00000004
	SUBMIT_SM_RESP    = 0x80000004
	DELIVER_SM        = 0x00000005
	DELIVER_SM_RESP   = 0x80000005
	UNBIND            = 0x00000006
	UNBIND_RESP       = 0x80000006
	ENQUIRE_LINK      = 0x00000015
	ENQUIRE_LINK_RESP = 0x80000015
)

// command status

const (
	STS_OK            = 0x00000000
	STS_INVALID_CMD   = 0x00000003
	STS_INV_BIND_STS  = 0x00000004
	STS_ALREADY_BOUND = 0x00000005
	STS_SYS_ERROR     = 0x00000008
)

// data coding

const (
	CODING_DEFAULT = 0x00
	CODING_UCS2    = 0x08
)

// optional parameters

const (
	TLV_RECEIPTED_MSG_ID = 0x001E
	TLV_MESSAGE_STATE    = 0x0427
)

type Session struct {
	SystemId  string
	Conn      net.Conn
	ReceiveMo bool
}

type Tlv struct {
	Tag   int
	Len   int
	Value []byte
}

type Smsc struct {
	Sessions      map[int]Session
	FailedSubmits bool
}

func NewSmsc(failedSubmits bool) Smsc {
	sessions := make(map[int]Session)
	return Smsc{sessions, failedSubmits}
}

func (smsc *Smsc) Start(port int, wg *sync.WaitGroup) {
	defer wg.Done()

	ln, err := net.Listen("tcp", fmt.Sprint(":", port))
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()

	log.Println("SMSC simulator listening on port", port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("error accepting new tcp connection %v", err)
		} else {
			go handleSmppConnection(smsc, conn)
		}
	}
}

func (smsc *Smsc) BoundSystemIds() []string {
	var systemIds []string
	for _, sess := range smsc.Sessions {
		systemId := sess.SystemId
		systemIds = append(systemIds, systemId)
	}
	return systemIds
}

func (smsc *Smsc) SendMoMessage(sender, recipient, message, systemId string) error {
	var session *Session = nil
	for _, sess := range smsc.Sessions {
		if systemId == sess.SystemId {
			session = &sess
			break
		}
	}

	if session == nil {
		log.Printf("Cannot send MO message to systemId: [%s]. No bound session found", systemId)
		return fmt.Errorf("No session found for systemId: [%s]", systemId)
	}

	if !session.ReceiveMo {
		log.Printf("Cannot send MO message to systemId: [%s]. Only RECEIVER and TRANSCEIVER sessions could receive MO messages", systemId)
		return fmt.Errorf("Only RECEIVER and TRANSCEIVER sessions could receive MO messages")
	}

	udhParts := toUdhParts(toUcs2Coding(message))
	esmClass := byte(0x00)
	if len(udhParts) > 1 {
		esmClass = 0x40
	}
	var tlvs []Tlv
	for i := range udhParts {
		pdu := deliverSmPDU(sender, recipient, udhParts[i], CODING_UCS2, rand.Int(), esmClass, tlvs)
		if _, err := session.Conn.Write(pdu); err != nil {
			log.Printf("Cannot send MO message to systemId: [%s]. Network error [%v]", systemId, err)
			return fmt.Errorf("Cannot send MO message. Network error")
		}
	}
	log.Printf("MO message to systemId: [%s] was successfully sent. Sender: [%s], recipient: [%s]", systemId, sender, recipient)
	return nil
}

// how to convert ints to and from bytes https://golang.org/pkg/encoding/binary/

func handleSmppConnection(smsc *Smsc, conn net.Conn) {
	sessionId := rand.Int()
	systemId := "anonymous"
	bound := false
	receiver := false

	defer delete(smsc.Sessions, sessionId)
	defer conn.Close()

	for {
		// read PDU header
		pduHeadBuf := make([]byte, 16)
		if _, err := io.ReadFull(conn, pduHeadBuf); err != nil {
			log.Printf("closing connection for system_id[%s] due %v\n", systemId, err)
			return
		}
		cmdLen := binary.BigEndian.Uint32(pduHeadBuf[0:])
		cmdId := binary.BigEndian.Uint32(pduHeadBuf[4:])
		// cmdSts := binary.BigEndian.Uint32(pduHeadBuf[8:])
		seqNum := binary.BigEndian.Uint32(pduHeadBuf[12:])

		var respBytes []byte

		switch cmdId {
		case BIND_RECEIVER, BIND_TRANSMITTER, BIND_TRANSCEIVER: // bind requests
			{
				pduBody := make([]byte, cmdLen-16)
				if _, err := io.ReadFull(conn, pduBody); err != nil {
					log.Printf("closing connection due %v\n", err)
					return
				}

				// find first null terminator
				idx := bytes.IndexByte(pduBody, byte(0))
				if idx == -1 {
					log.Printf("invalid pdu_body. cannot find system_id. closing connection")
					return
				}
				systemId = string(pduBody[:idx])
				log.Printf("bind request from system_id[%s]\n", systemId)

				respCmdId := 2147483648 + cmdId // hack to calc resp cmd id

				if bound {
					respBytes = headerPDU(respCmdId, STS_ALREADY_BOUND, seqNum)
					log.Printf("[%s] already has bound session", systemId)
				} else {
					receiveMo := cmdId == BIND_RECEIVER || cmdId == BIND_TRANSCEIVER
					smsc.Sessions[sessionId] = Session{systemId, conn, receiveMo}
					respBytes = stringBodyPDU(respCmdId, STS_OK, seqNum, "smscsim")
					bound = true
					receiver = cmdId == BIND_RECEIVER
				}
			}
		case UNBIND: // unbind request
			{
				log.Printf("unbind request from system_id[%s]\n", systemId)
				respBytes = headerPDU(UNBIND_RESP, STS_OK, seqNum)
				bound = false
				systemId = "anonymous"
			}
		case ENQUIRE_LINK: // enquire_link
			{
				log.Printf("enquire_link from system_id[%s]\n", systemId)
				respBytes = headerPDU(ENQUIRE_LINK_RESP, STS_OK, seqNum)
			}
		case SUBMIT_SM: // submit_sm
			{
				pduBody := make([]byte, cmdLen-16)
				if _, err := io.ReadFull(conn, pduBody); err != nil {
					log.Printf("error reading submit_sm body for %s due %v. closing connection", systemId, err)
					return
				}
				log.Printf("submit_sm from system_id[%s]\n", systemId)

				if receiver {
					respBytes = headerPDU(SUBMIT_SM_RESP, STS_INV_BIND_STS, seqNum)
					log.Printf("error handling submit_sm from system_id[%s]. session with bind type RECEIVER cannot send requests", systemId)
					break
				}

				idxCounter := 0
				nullTerm := byte(0)

				srvTypeEndIdx := bytes.IndexByte(pduBody, nullTerm)
				if srvTypeEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				idxCounter = idxCounter + srvTypeEndIdx
				idxCounter = idxCounter + 3 // skip src ton and npi

				srcAddrEndIdx := bytes.IndexByte(pduBody[idxCounter:], nullTerm)
				if srcAddrEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				srcAddr := string(pduBody[idxCounter : idxCounter+srcAddrEndIdx])
				idxCounter = idxCounter + srcAddrEndIdx
				idxCounter = idxCounter + 3 // skip dest ton and npi

				destAddrEndIdx := bytes.IndexByte(pduBody[idxCounter:], nullTerm)
				if destAddrEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				destAddr := string(pduBody[idxCounter : idxCounter+destAddrEndIdx])
				idxCounter = idxCounter + destAddrEndIdx
				idxCounter = idxCounter + 4 // skip esm_class, protocol_id, priority_flag

				schedEndIdx := bytes.IndexByte(pduBody[idxCounter:], nullTerm)
				if schedEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				idxCounter = idxCounter + schedEndIdx
				idxCounter = idxCounter + 1 // next is validity period

				validityEndIdx := bytes.IndexByte(pduBody[idxCounter:], nullTerm)
				if validityEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				idxCounter = idxCounter + validityEndIdx
				registeredDlr := pduBody[idxCounter+1] // registered_delivery is next field after the validity_period

				// prepare submit_sm_resp
				msgId := strconv.Itoa(rand.Int())

				if smsc.FailedSubmits && seqNum%2 == 0 {
					// return error response
					respBytes = headerPDU(SUBMIT_SM_RESP, STS_SYS_ERROR, seqNum)
				} else {
					respBytes = stringBodyPDU(SUBMIT_SM_RESP, STS_OK, seqNum, msgId)
					// send DLR if necessary
					if registeredDlr != 0 {
						go func() {
							time.Sleep(2000 * time.Millisecond)
							now := time.Now()
							dlr := deliveryReceiptPDU(destAddr, srcAddr, msgId, now, now, smsc.FailedSubmits)
							if _, err := conn.Write(dlr); err != nil {
								log.Printf("error sending delivery receipt to system_id[%s] due %v.", systemId, err)
								return
							} else {
								log.Printf("delivery receipt for message [%s] was send to system_id[%s]", msgId, systemId)
							}
						}()
					}
				}
			}
		case DELIVER_SM_RESP: // deliver_sm_resp
			{
				if cmdLen > 16 {
					buf := make([]byte, cmdLen-16)
					if _, err := io.ReadFull(conn, buf); err != nil {
						log.Printf("error reading deliver_sm_resp for %s due %v. closing connection", systemId, err)
						return
					}
				}
				log.Println("deliver_sm_resp from", systemId)
			}
		default:
			{
				if cmdLen > 16 {
					buf := make([]byte, cmdLen-16)
					if _, err := io.ReadFull(conn, buf); err != nil {
						log.Printf("error reading pdu for %s due %v. closing connection", systemId, err)
						return
					}
				}
				log.Printf("unsupported pdu cmd_id(%d) from %s", cmdId, systemId)
				// generic nack packet with status "Invalid Command ID"
				respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
			}
		}

		if _, err := conn.Write(respBytes); err != nil {
			log.Printf("error sending response to system_id[%s] due %v. closing connection", systemId, err)
			return
		}
	}
}

func headerPDU(cmdId, cmdSts, seqNum uint32) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint32(buf[0:], 16)
	binary.BigEndian.PutUint32(buf[4:], cmdId)
	binary.BigEndian.PutUint32(buf[8:], cmdSts)
	binary.BigEndian.PutUint32(buf[12:], seqNum)
	return buf
}

func stringBodyPDU(cmdId, cmdSts, seqNum uint32, body string) []byte {
	cmdLen := 16 + len(body) + 1 // 16 for header + body length with null terminator
	buf := make([]byte, 16)
	binary.BigEndian.PutUint32(buf[0:], uint32(cmdLen))
	binary.BigEndian.PutUint32(buf[4:], cmdId)
	binary.BigEndian.PutUint32(buf[8:], cmdSts)
	binary.BigEndian.PutUint32(buf[12:], seqNum)
	buf = append(buf, body...)
	buf = append(buf, "\x00"...)
	return buf
}

const DLR_RECEIPT_FORMAT = "id:%s sub:001 dlvrd:001 submit date:%s done date:%s stat:DELIVRD err:000 Text:..."
const DLR_RECEIPT_FORMAT_FAILED = "id:%s sub:001 dlvrd:000 submit date:%s done date:%s stat:UNDELIV err:069 Text:..."

func deliveryReceiptPDU(src, dst, msgId string, submitDate, doneDate time.Time, failedDeliv bool) []byte {
	sbtDateFrmt := submitDate.Format("0601021504")
	doneDateFrmt := doneDate.Format("0601021504")
	dlrFmt := DLR_RECEIPT_FORMAT
	msgState := []byte{2}
	if failedDeliv {
		dlrFmt = DLR_RECEIPT_FORMAT_FAILED
		msgState = []byte{5}
	}
	deliveryReceipt := fmt.Sprintf(dlrFmt, msgId, sbtDateFrmt, doneDateFrmt)
	var tlvs []Tlv

	// receipted_msg_id TLV
	var rcptMsgIdBuf bytes.Buffer
	rcptMsgIdBuf.WriteString(msgId)
	rcptMsgIdBuf.WriteByte(0) // null terminator
	receiptMsgId := Tlv{TLV_RECEIPTED_MSG_ID, rcptMsgIdBuf.Len(), rcptMsgIdBuf.Bytes()}
	tlvs = append(tlvs, receiptMsgId)

	// message_state TLV
	msgStateTlv := Tlv{TLV_MESSAGE_STATE, 1, msgState}
	tlvs = append(tlvs, msgStateTlv)

	return deliverSmPDU(src, dst, []byte(deliveryReceipt), CODING_DEFAULT, rand.Int(), 0x04, tlvs)
}

func deliverSmPDU(sender, recipient string, shortMessage []byte, coding byte, seqNum int, esmClass byte, tlvs []Tlv) []byte {
	// header without cmd_len
	header := make([]byte, 12)
	binary.BigEndian.PutUint32(header[0:], uint32(DELIVER_SM))
	binary.BigEndian.PutUint32(header[4:], uint32(0))
	binary.BigEndian.PutUint32(header[8:], uint32(seqNum)) // rand seq num

	// pdu body buffer
	var buf bytes.Buffer
	buf.Write(header)

	buf.WriteString("smscsim")
	buf.WriteByte(0) // null term

	buf.WriteByte(0) // src ton
	buf.WriteByte(0) // src npi
	if sender == "" {
		buf.WriteByte(0)
	} else {
		buf.WriteString(sender)
		buf.WriteByte(0)
	}

	buf.WriteByte(0) // dest ton
	buf.WriteByte(0) // dest npi
	if recipient == "" {
		buf.WriteByte(0)
	} else {
		buf.WriteString(recipient)
		buf.WriteByte(0)
	}

	buf.WriteByte(esmClass) // esm class
	buf.WriteByte(0)        // protocol id
	buf.WriteByte(0)        // priority flag
	buf.WriteByte(0)        // sched delivery time
	buf.WriteByte(0)        // validity period
	buf.WriteByte(0)        // registered delivery
	buf.WriteByte(0)        // replace if present
	buf.WriteByte(coding)   // data coding
	buf.WriteByte(0)        // def msg id

	smLen := len(shortMessage)
	buf.WriteByte(byte(smLen))
	buf.Write(shortMessage)

	for _, t := range tlvs {
		tlvBytes := make([]byte, 4)
		binary.BigEndian.PutUint16(tlvBytes[0:], uint16(t.Tag))
		binary.BigEndian.PutUint16(tlvBytes[2:], uint16(t.Len))
		buf.Write(tlvBytes)
		buf.Write(t.Value)
	}

	// calc cmd lenth and append to the begining
	cmdLen := buf.Len() + 4 // +4 for cmdLen field itself
	cmdLenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(cmdLenBytes[0:], uint32(cmdLen))

	var deliverSm bytes.Buffer
	deliverSm.Write(cmdLenBytes)
	deliverSm.Write(buf.Bytes())

	return deliverSm.Bytes()
}

func toUcs2Coding(input string) []byte {
	// not most elegant implementation, but ok for testing purposes
	l := utf8.RuneCountInString(input)
	buf := make([]byte, l*2) // two bytes per character
	idx := 0
	for len(input) > 0 {
		r, s := utf8.DecodeRuneInString(input)
		if r <= 65536 {
			binary.BigEndian.PutUint16(buf[idx:], uint16(r))
		} else {
			binary.BigEndian.PutUint16(buf[idx:], uint16(63)) // question mark
		}
		input = input[s:]
		idx += 2
	}
	return buf
}

func toUdhParts(longMsg []byte) [][]byte {
	msgLen := len(longMsg)
	if msgLen <= 140 { // max len for message field in pdu
		// one part is enough
		return [][]byte{longMsg}
	}
	maxUdhContentLen := 134
	c := int(math.Ceil(float64(msgLen) / float64(maxUdhContentLen)))
	parts := make([][]byte, c)
	for i := 0; i < c; i++ {
		si := i * maxUdhContentLen
		ei := int(math.Min(float64(si+maxUdhContentLen), float64(msgLen)))
		partLen := ei - si
		part := make([]byte, partLen+6) // plus 6 for udh headers
		part[0] = 0x05
		part[1] = 0x00
		part[2] = 0x03
		part[3] = 0x01 // maybe accept id as method argument?
		part[4] = byte(c)
		part[5] = byte(i + 1)
		copy(part[6:], longMsg[si:ei])
		parts[i] = part
	}
	return parts
}
