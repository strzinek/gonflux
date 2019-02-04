package main

import (
	"bytes"
	"encoding/binary"
	"github.com/namsral/flag"
	"fmt"
	"encoding/json"
	"log"
	"net"
	"time"
	"sync"
)

// NetFlow v5 implementation

type header struct {
	Version          uint16
	FlowRecords      uint16
	Uptime           uint32
	UnixSec          uint32
	UnixNsec         uint32
	FlowSeqNum       uint32
	EngineType       uint8
	EngineID         uint8
	SamplingInterval uint16
}

type binaryRecord struct {
	Ipv4SrcAddrInt uint32
	Ipv4DstAddrInt uint32
	Ipv4NextHopInt uint32
	InputSnmp      uint16
	OutputSnmp     uint16
	InPkts         uint32
	InBytes        uint32
	FirstInt	   uint32
	LastInt		   uint32
	L4SrcPort      uint16
	L4DstPort      uint16
	_              uint8
	TCPFlags       uint8
	Protocol       uint8
	SrcTos         uint8
	SrcAs          uint16
	DstAs          uint16
	SrcMask        uint8
	DstMask        uint8
	_              uint16
}

type decodedRecord struct {
	header
	binaryRecord

	Host              string
	SamplingAlgorithm uint8
	Ipv4SrcAddr       string
	Ipv4DstAddr       string
	Ipv4NextHop       string
	SrcHostName       string
	DstHostName       string
	Duration          uint16
}

func intToIPv4Addr(intAddr uint32) net.IP {
	return net.IPv4(
		byte(intAddr>>24),
		byte(intAddr>>16),
		byte(intAddr>>8),
		byte(intAddr))
}

func decodeRecord(header *header, binRecord *binaryRecord, remoteAddr *net.UDPAddr) decodedRecord {

	decodedRecord := decodedRecord{

		Host: remoteAddr.IP.String(),

		header: *header,

		binaryRecord: *binRecord,

		Ipv4SrcAddr: intToIPv4Addr(binRecord.Ipv4SrcAddrInt).String(),
		Ipv4DstAddr: intToIPv4Addr(binRecord.Ipv4DstAddrInt).String(),
		Ipv4NextHop: intToIPv4Addr(binRecord.Ipv4NextHopInt).String(),
		Duration: uint16((binRecord.LastInt-binRecord.FirstInt)/1000),
	}

  // LookupAddr
	decodedRecord.SrcHostName = lookUpWithCache(decodedRecord.Ipv4SrcAddr)
	decodedRecord.DstHostName = lookUpWithCache(decodedRecord.Ipv4DstAddr)

	// decode sampling info
	decodedRecord.SamplingAlgorithm = uint8(0x3 & (decodedRecord.SamplingInterval >> 14))
	decodedRecord.SamplingInterval = 0x3fff & decodedRecord.SamplingInterval

	return decodedRecord
}

func pipeOutputToStdout(outputChannel chan decodedRecord) {
	var record decodedRecord
	for {
		record = <-outputChannel
		out, _ := json.Marshal(record)
		fmt.Println(string(out))
	}
}

type cacheRecord struct {
	Hostname string
	timeout time.Time
}

var (
	cache = map[string]cacheRecord{}
	cacheMutex = sync.RWMutex{}
)

func lookUpWithCache (ipAddr string) string {
	hostname :=ipAddr
	cacheMutex.Lock()
	hostnameFromCache :=cache[ipAddr]
	cacheMutex.Unlock()
	if (hostnameFromCache == cacheRecord{} || time.Now().After(hostnameFromCache.timeout)) {
		hostTemp, err := net.LookupAddr(ipAddr)
		if err == nil {
			hostname = hostTemp[0]
		}
		cacheMutex.Lock()
		cache[ipAddr] = cacheRecord{hostname,time.Now().AddDate(0,0,1)}
		cacheMutex.Unlock()
	} else {
		hostname = hostnameFromCache.Hostname
	}
	return hostname
}

func formatLineProtocol(record decodedRecord) []byte {
	return []byte(fmt.Sprintf("netflow,host=%s,srcAddr=%s,dstAddr=%s,srcHostName=%s,dstHostName=%s,protocol=%d,srcPort=%d,dstPort=%d,input=%d,output=%d inBytes=%d,inPackets=%d,duration=%d %d",
		record.Host,record.Ipv4SrcAddr,record.Ipv4DstAddr,record.SrcHostName,record.DstHostName,record.Protocol,record.L4SrcPort,record.L4DstPort,record.InputSnmp,record.OutputSnmp,
		record.InBytes,record.InPkts,record.Duration,
		uint64((uint64(record.UnixSec)*uint64(1000000000))+uint64(record.UnixNsec))))
}

func pipeOutputToUDPSocket(outputChannel chan decodedRecord, targetAddr string) {
	/* Setting-up the socket to send data */

	remote, err := net.ResolveUDPAddr("udp", targetAddr)
	if err != nil {
		log.Printf("Name resolution failed: %v\n", err)
	} else {

		for {
			connection, err := net.DialUDP("udp", nil, remote)
			defer connection.Close()
			if err != nil {
				log.Printf("Connection failed: %v\n", err)
			} else {
				var record decodedRecord
				for {
					record = <-outputChannel
					var buf = formatLineProtocol(record)
					conn := connection
					conn.SetDeadline(time.Now().Add(3 * time.Second))
					_, err := conn.Write(buf)
					if err != nil {
						log.Printf("Send Error: %v\n", err)
						break
					}
				}
			}
		}
	}
}

func handlePacket(buf *bytes.Buffer, remoteAddr *net.UDPAddr, outputChannel chan decodedRecord) {
	header := header{}
	err := binary.Read(buf, binary.BigEndian, &header)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {

		for i := 0; i < int(header.FlowRecords); i++ {
			record := binaryRecord{}
			err := binary.Read(buf, binary.BigEndian, &record)
			if err != nil {
				log.Printf("binary.Read failed: %v\n", err)
				break
			}

			decodedRecord := decodeRecord(&header, &record, remoteAddr)
			outputChannel <- decodedRecord
		}
	}
}

func main() {
	/* Parse command-line arguments */
	var (
		inSource               string
		outMethod              string
		outDestination         string
		receiveBufferSizeBytes int
	)
	flag.StringVar(&inSource, "in", "0.0.0.0:2055", "Address and port to listen NetFlow packets")
	flag.StringVar(&outMethod, "method", "stdout", "Output method: stdout, udp")
	flag.StringVar(&outDestination, "out", "", "Address and port of influxdb to send decoded data")
	flag.IntVar(&receiveBufferSizeBytes, "buffer", 212992, "Size of RxQueue, i.e. value for SO_RCVBUF in bytes")
	flag.Parse()

	/* Create output pipe */
	outputChannel := make(chan decodedRecord, 100)
	switch outMethod {
	case "stdout":
		go pipeOutputToStdout(outputChannel)
	case "udp":
		go pipeOutputToUDPSocket(outputChannel, outDestination)
	default:
		log.Fatalf("Unknown schema: %v\n", outMethod)

	}

	/* Start listerning on the specified port */
	log.Printf("Start listening on %v and sending to %v %v\n", inSource, outMethod, outDestination)
	addr, err := net.ResolveUDPAddr("udp", inSource)
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}

	for {
		conn, err := net.ListenUDP("udp", addr)
		defer conn.Close()
		if err != nil {
			log.Println(err)
		} else {
			err = conn.SetReadBuffer(receiveBufferSizeBytes)
			if err != nil {
				log.Println(err)
			} else {

				/* Infinite-loop for reading packets */
				for {
					buf := make([]byte, 4096)
					rlen, remote, err := conn.ReadFromUDP(buf)

					if err != nil {
						log.Printf("Error: %v\n", err)
					} else {

						stream := bytes.NewBuffer(buf[:rlen])

						go handlePacket(stream, remote, outputChannel)
					}
				}
			}
		}
		defer conn.Close()

	}
}
