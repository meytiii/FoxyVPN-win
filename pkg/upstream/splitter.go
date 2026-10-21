package upstream

import (
	"net"
	"sync"
)

type TlsSplitMode int

const (
	SplitOff TlsSplitMode = iota
	SplitFirstByte
	SplitSni
)

// LooksLikeClientHello checks if the byte buffer starts with a TLS 1.0-1.3 ClientHello record.
func LooksLikeClientHello(b []byte) bool {
	if len(b) < 6 {
		return false
	}
	return b[0] == 0x16 && b[1] == 0x03 && b[5] == 0x01
}

// FindSniOffset traverses a TLS ClientHello to pinpoint the start and end offsets of the SNI hostname.
func FindSniOffset(b []byte) (start int, end int, found bool) {
	if len(b) < 44 || b[0] != 0x16 {
		return 0, 0, false
	}
	curr := 5
	if b[curr] != 0x01 {
		return 0, 0, false
	}
	curr += 4       // Handshake header length
	curr += 2 + 32  // Version (2) + Random (32)
	if curr >= len(b) {
		return 0, 0, false
	}

	sessionIDLen := int(b[curr])
	curr += 1 + sessionIDLen
	if curr+2 > len(b) {
		return 0, 0, false
	}

	cipherSuitesLen := int(b[curr])<<8 | int(b[curr+1])
	curr += 2 + cipherSuitesLen
	if curr >= len(b) {
		return 0, 0, false
	}

	compressionMethodsLen := int(b[curr])
	curr += 1 + compressionMethodsLen
	if curr+2 > len(b) {
		return 0, 0, false
	}

	extensionsLen := int(b[curr])<<8 | int(b[curr+1])
	extEnd := curr + 2 + extensionsLen
	curr += 2
	if extEnd > len(b) {
		extEnd = len(b)
	}

	for curr+4 <= extEnd {
		extType := int(b[curr])<<8 | int(b[curr+1])
		extLen := int(b[curr+2])<<8 | int(b[curr+3])
		body := curr + 4
		if body+extLen > extEnd {
			return 0, 0, false
		}

		if extType == 0 { // Server Name Indication (SNI)
			if extLen < 2 {
				return 0, 0, false
			}
			listLen := int(b[body])<<8 | int(b[body+1])
			list := body + 2
			if list+listLen > body+extLen {
				return 0, 0, false
			}
			if listLen < 3 || b[list] != 0 {
				return 0, 0, false
			}
			nameLen := int(b[list+1])<<8 | int(b[list+2])
			s := list + 3
			e := s + nameLen
			if e > list+listLen || e > len(b) {
				return 0, 0, false
			}
			return s, e, true
		}
		curr = body + extLen
	}
	return 0, 0, false
}

// SplitBytes segments a TLS ClientHello packet according to the specified split mode.
func SplitBytes(b []byte, mode TlsSplitMode) [][]byte {
	if len(b) < 2 {
		return [][]byte{b}
	}

	switch mode {
	case SplitFirstByte:
		return [][]byte{
			b[:1],
			b[1:],
		}

	case SplitSni:
		start, _, found := FindSniOffset(b)
		if !found || start <= 0 || start >= len(b) {
			return [][]byte{b}
		}
		return [][]byte{
			b[:start],
			b[start : start+1],
			b[start+1:],
		}

	default:
		return [][]byte{b}
	}
}

// SplitConn wraps a net.Conn to perform packet splitting on the first TLS ClientHello write.
type SplitConn struct {
	net.Conn
	mode      TlsSplitMode
	mu        sync.Mutex
	splitDone bool
}

// NewSplitConn wraps a raw TCP connection with client hello splitting capabilities.
func NewSplitConn(conn net.Conn, mode TlsSplitMode) *SplitConn {
	return &SplitConn{
		Conn: conn,
		mode: mode,
	}
}

func (s *SplitConn) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.splitDone || s.mode == SplitOff || !LooksLikeClientHello(b) {
		return s.Conn.Write(b)
	}

	s.splitDone = true
	segments := SplitBytes(b, s.mode)
	if len(segments) <= 1 {
		return s.Conn.Write(b)
	}

	if tcpConn, ok := s.Conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	totalWritten := 0
	for _, seg := range segments {
		n, err := s.Conn.Write(seg)
		totalWritten += n
		if err != nil {
			return totalWritten, err
		}
	}

	return totalWritten, nil
}
