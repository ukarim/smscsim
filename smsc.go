package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
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
	STS_OK          = 0x00000000
	STS_INVALID_CMD = 0x00000003
)

const (
	TLV_RECEIPTED_MSG_ID = 0x001E
	TLV_MESSAGE_STATE    = 0x0427
)

type Session struct {
	SystemId string
	Conn     net.Conn
}

type Tlv struct {
	Tag   int
	Len   int
	Value []byte
}

type Smsc struct {
	Sessions map[int]Session
}

func NewSmsc() Smsc {
	sessions := make(map[int]Session)
	return Smsc{sessions}
}

func (smsc *Smsc) Start(port int, wg sync.WaitGroup) {
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
	var conn net.Conn
	for _, sess := range smsc.Sessions {
		if systemId == sess.SystemId {
			conn = sess.Conn
		}
	}

	if conn == nil {
		log.Printf("Cannot send MO message to systemId: [%s]. No bound session found", systemId)
		return fmt.Errorf("No session found for systemId: [%s]", systemId)
	}

	// TODO implement UDH for large messages
	shortMsg := truncateString(message, 70) // just truncate to 70 symbols
	var tlvs []Tlv
	moMessage := deliverSmPDU(sender, recipient, shortMsg, rand.Int(), tlvs)
	if _, err := conn.Write(moMessage); err != nil {
		log.Printf("Cannot send MO message to systemId: [%s]. Network error [%v]", systemId, err)
		return fmt.Errorf("Cannot send MO message. Network error")
	} else {
		log.Printf("MO message to systemId: [%s] was successfully sent. Sender: [%s], recipient: [%s]", systemId, sender, recipient)
		return nil
	}
}

// how to convert ints to and from bytes https://golang.org/pkg/encoding/binary/

func handleSmppConnection(smsc *Smsc, conn net.Conn) {
	sessionId := rand.Int()
	systemId := "anonymous"
	stopLoop := false

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
				idx := bytes.Index(pduBody, []byte("\x00"))
				if idx == -1 {
					log.Printf("invalid pdu_body. cannot find system_id. closing connection")
					return
				}
				systemId = string(pduBody[:idx])
				smsc.Sessions[sessionId] = Session{systemId, conn}
				log.Printf("bind request from system_id[%s]\n", systemId)

				respCmdId := 2147483648 + cmdId // hack to calc resp cmd id
				respBytes = stringBodyPDU(respCmdId, STS_OK, seqNum, "smscsim")
			}
		case UNBIND: // unbind request
			{
				log.Printf("unbind request from system_id[%s]\n", systemId)
				respBytes = headerPDU(UNBIND_RESP, STS_OK, seqNum)
				stopLoop = true
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

				idxCounter := 0
				nullTerm := []byte("\x00")

				srvTypeEndIdx := bytes.Index(pduBody, nullTerm)
				if srvTypeEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				idxCounter = idxCounter + srvTypeEndIdx
				idxCounter = idxCounter + 3 // skip src ton and npi

				srcAddrEndIdx := bytes.Index(pduBody[idxCounter:], nullTerm)
				if srcAddrEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				idxCounter = idxCounter + srcAddrEndIdx
				idxCounter = idxCounter + 3 // skip dest ton and npi

				destAddrEndIdx := bytes.Index(pduBody[idxCounter:], nullTerm)
				if destAddrEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				idxCounter = idxCounter + destAddrEndIdx
				idxCounter = idxCounter + 4 // skip esm_class, protocol_id, priority_flag

				schedEndIdx := bytes.Index(pduBody[idxCounter:], nullTerm)
				if schedEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				idxCounter = idxCounter + schedEndIdx
				idxCounter = idxCounter + 1 // next is validity period

				validityEndIdx := bytes.Index(pduBody[idxCounter:], nullTerm)
				if validityEndIdx == -1 {
					respBytes = headerPDU(GENERIC_NACK, STS_INVALID_CMD, seqNum)
					break
				}
				idxCounter = idxCounter + validityEndIdx
				registeredDlr := pduBody[idxCounter+1] // registered_delivery is next field after the validity_period

				// prepare submit_sm_resp
				// msgId := strconv.Itoa(rand.Int())
				// respBytes = stringBodyPDU(SUBMIT_SM_RESP, STS_OK, seqNum, msgId)

				// deal with message id to simulate cmi
				// msgId need format to hex in submit_sm
				id := rand.Int()
				msgId := strconv.Itoa(id)
				msgIdHex := strconv.FormatInt(int64(id), 16)
				respBytes = stringBodyPDU(SUBMIT_SM_RESP, STS_OK, seqNum, msgIdHex)

				if registeredDlr != 0 {
					go func() {
						time.Sleep(2000 * time.Millisecond)
						now := time.Now()
						dlr := deliveryReceiptPDU(msgId, now, now)
						if _, err := conn.Write(dlr); err != nil {
							log.Printf("error sending delivery receipt to system_id[%s] due %v.", systemId, err)
							return
						} else {
							log.Printf("delivery receipt for message [%s] was send to system_id[%s]", msgId, systemId)
						}
					}()
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

		if stopLoop {
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

const DELIVERY_RECEIPT_FORMAT = "id:%s sub:001 dlvrd:001 submit date:%s done date:%s stat:DELIVRD err:000 Text:..."

func deliveryReceiptPDU(msgId string, submitDate, doneDate time.Time) []byte {
	sbtDateFrmt := submitDate.Format("0601021504")
	doneDateFrmt := doneDate.Format("0601021504")
	deliveryReceipt := fmt.Sprintf(DELIVERY_RECEIPT_FORMAT, msgId, sbtDateFrmt, doneDateFrmt)
	var tlvs []Tlv

	// receipted_msg_id TLV
	var rcptMsgIdBuf bytes.Buffer
	rcptMsgIdBuf.WriteString(msgId)
	rcptMsgIdBuf.WriteByte(0) // null terminator
	receiptMsgId := Tlv{TLV_RECEIPTED_MSG_ID, rcptMsgIdBuf.Len(), rcptMsgIdBuf.Bytes()}
	tlvs = append(tlvs, receiptMsgId)

	// message_state TLV
	msgStateTlv := Tlv{TLV_MESSAGE_STATE, 1, []byte{2}} // 2 - delivered
	tlvs = append(tlvs, msgStateTlv)

	return deliverSmPDU("", "", deliveryReceipt, rand.Int(), tlvs)
}

func deliverSmPDU(sender, recipient, shortMessage string, seqNum int, tlvs []Tlv) []byte {
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

	buf.WriteByte(0) // esm class
	buf.WriteByte(0) // protocol id
	buf.WriteByte(0) // priority flag
	buf.WriteByte(0) // sched delivery time
	buf.WriteByte(0) // validity period
	buf.WriteByte(0) // registered delivery
	buf.WriteByte(0) // replace if present
	buf.WriteByte(0) // data coding
	buf.WriteByte(0) // def msg id

	smLen := len(shortMessage)
	buf.WriteByte(byte(smLen))
	buf.WriteString(shortMessage)

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

func truncateString(input string, maxLen int) string {
	result := input
	if len(input) > maxLen {
		result = input[0:maxLen]
	}
	return result
}
