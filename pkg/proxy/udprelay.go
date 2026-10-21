package proxy

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"foxyvpn/pkg/dns"
)

type DnsRelayFunc func(ctx context.Context, query []byte) ([]byte, error)

// UdpDnsRelay manages a SOCKS5 UDP ASSOCIATE relay that forwards port 53 DNS over DoH.
type UdpDnsRelay struct {
	udpConn     *net.UDPConn
	clientAddr  *net.UDPAddr
	tcpConn     net.Conn
	dohClient   *dns.DohClient
	dohURL      string
	dnsFallback DnsRelayFunc
	closed      chan struct{}
	closeOnce   sync.Once
}

func NewUdpDnsRelay(tcpConn net.Conn, dohClient *dns.DohClient, dohURL string) (*UdpDnsRelay, error) {
	// Bind to local loopback on ephemeral port
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind UDP relay socket: %w", err)
	}

	relay := &UdpDnsRelay{
		udpConn:   udpConn,
		tcpConn:   tcpConn,
		dohClient: dohClient,
		dohURL:    dohURL,
		closed:    make(chan struct{}),
	}

	return relay, nil
}

func (r *UdpDnsRelay) BoundPort() int {
	return r.udpConn.LocalAddr().(*net.UDPAddr).Port
}

func (r *UdpDnsRelay) Start() {
	go r.readLoop()
	go r.watchTcpConnection()
}

func (r *UdpDnsRelay) watchTcpConnection() {
	// When the control TCP connection closes, the UDP association must terminate.
	buf := make([]byte, 1)
	_, _ = r.tcpConn.Read(buf)
	r.Close()
}

func (r *UdpDnsRelay) Close() {
	r.closeOnce.Do(func() {
		close(r.closed)
		if r.udpConn != nil {
			_ = r.udpConn.Close()
		}
	})
}

func (r *UdpDnsRelay) readLoop() {
	buf := make([]byte, 4096+256)
	for {
		select {
		case <-r.closed:
			return
		default:
		}

		_ = r.udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, remoteAddr, err := r.udpConn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		if n < 10 {
			continue
		}

		// Save/verify client address
		if r.clientAddr == nil {
			r.clientAddr = remoteAddr
		} else if remoteAddr.IP.String() != r.clientAddr.IP.String() {
			continue // Drop datagrams from foreign sources for security
		}

		// SOCKS5 UDP header:
		// +----+------+------+----------+----------+----------+
		// |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
		// +----+------+------+----------+----------+----------+
		// | 2  |  1   |  1   | Variable |    2     | Variable |
		// +----+------+------+----------+----------+----------+
		frag := buf[2]
		if frag != 0x00 {
			continue // Fragmentation not supported
		}

		atyp := buf[3]
		var dataOffset int
		var dstPort int

		switch atyp {
		case 0x01: // IPv4
			dataOffset = 4 + 4 + 2
			dstPort = int(binary.BigEndian.Uint16(buf[8:10]))
		case 0x03: // FQDN
			domainLen := int(buf[4])
			dataOffset = 5 + domainLen + 2
			dstPort = int(binary.BigEndian.Uint16(buf[5+domainLen : dataOffset]))
		case 0x04: // IPv6
			dataOffset = 4 + 16 + 2
			dstPort = int(binary.BigEndian.Uint16(buf[20:22]))
		default:
			continue
		}

		if n <= dataOffset {
			continue
		}

		dnsQuery := buf[dataOffset:n]
		headerCopy := make([]byte, dataOffset)
		copy(headerCopy, buf[:dataOffset])

		go r.handleDnsPacket(remoteAddr, headerCopy, dnsQuery, dstPort)
	}
}

func (r *UdpDnsRelay) handleDnsPacket(clientAddr *net.UDPAddr, socksHeader, dnsQuery []byte, dstPort int) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	var answer []byte
	var err error

	if r.dohClient != nil && r.dohURL != "" {
		answer, err = r.dohClient.QueryDoh(ctx, r.dohURL, dnsQuery)
	}

	if err != nil || len(answer) == 0 {
		return
	}

	// Wrap answer in SOCKS5 UDP header and send back to client
	reply := append(socksHeader, answer...)
	_, _ = r.udpConn.WriteToUDP(reply, clientAddr)
}
