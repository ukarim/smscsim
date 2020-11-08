package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
)

// command id
const (
	GENERIC_NACK      = 0x80000000
	BIND_RECEIVER     = 0x00000001
	BIND_TRANSMITTER  = 0x00000002
	BIND_TRANSCEIVER  = 0x00000009
	SUBMIT_SM         = 0x00000004
	SUBMIT_SM_RESP    = 0x80000004
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

type Session struct {
	SystemId string
	Conn     net.Conn
}

type Smsc struct {
	Sessions map[int]Session
}

func NewSmsc() Smsc {
	sessions := make(map[int]Session)
	return Smsc{sessions}
}

func (smsc *Smsc) Start(wg sync.WaitGroup) {
	defer wg.Done()

	ln, err := net.Listen("tcp", ":2775")
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()

	log.Println("SMSC simulator listening on port 2775")
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
	systemIds := make([]string, len(smsc.Sessions))
	for id, sess := range smsc.Sessions {
		systemId := sess.SystemId + "-" + strconv.Itoa(id)
		systemIds = append(systemIds, systemId)
	}
	return systemIds
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
				respBytes = stringBodyPduBytes(respCmdId, STS_OK, seqNum, "smscsim")
			}
		case UNBIND: // unbind request
			{
				log.Printf("unbind request from system_id[%s]\n", systemId)
				respBytes = pduHeaderBytes(UNBIND_RESP, STS_OK, seqNum)
				stopLoop = true
			}
		case ENQUIRE_LINK: // enquire_link
			{
				log.Printf("enquire_link from system_id[%s]\n", systemId)
				respBytes = pduHeaderBytes(ENQUIRE_LINK_RESP, STS_OK, seqNum)
			}
		case SUBMIT_SM: // submit_sm
			{
				buf := make([]byte, cmdLen-16)
				if _, err := io.ReadFull(conn, buf); err != nil {
					log.Printf("error reading submit_sm body for %s due %v. closing connection", systemId, err)
					return
				}
				log.Printf("submit_sm from system_id[%s]\n", systemId)
				msgId := rand.Int()
				respBytes = stringBodyPduBytes(SUBMIT_SM_RESP, STS_OK, seqNum, strconv.Itoa(msgId))
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
				respBytes = pduHeaderBytes(GENERIC_NACK, STS_INVALID_CMD, seqNum)
				stopLoop = true
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

func pduHeaderBytes(cmdId, cmdSts, seqNum uint32) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint32(buf[0:], 16)
	binary.BigEndian.PutUint32(buf[4:], cmdId)
	binary.BigEndian.PutUint32(buf[8:], cmdSts)
	binary.BigEndian.PutUint32(buf[12:], seqNum)
	return buf
}

func stringBodyPduBytes(cmdId, cmdSts, seqNum uint32, body string) []byte {
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
