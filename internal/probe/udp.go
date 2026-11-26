package probe

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// udpExchange is one request datagram plus a check that recognises its reply.
// UDP has no handshake, so a probe only gets a latency when it speaks the
// protocol the target port expects.
type udpExchange struct {
	name    string
	request []byte
	match   func(reply []byte) (ok bool, note string)
}

const (
	udpPortDNS = 53
	udpPortNTP = 123
)

func udpExchangeForPort(port int) (udpExchange, error) {
	switch port {
	case udpPortDNS:
		return newDNSExchange()
	case udpPortNTP:
		return newNTPExchange()
	default:
		return genericUDPExchange(), nil
	}
}

func probeUDP(ctx context.Context, spec Spec, result *Result) error {
	exchange, err := udpExchangeForPort(spec.Port)
	if err != nil {
		return err
	}
	return probeUDPExchange(ctx, spec, result, exchange)
}

func probeUDPExchange(ctx context.Context, spec Spec, result *Result, exchange udpExchange) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(spec.Host, strconv.Itoa(spec.Port)))
	if err != nil {
		return err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	}

	if _, err := conn.Write(exchange.request); err != nil {
		return err
	}

	buffer := make([]byte, 1500)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) || isTimeout(err) {
				result.Status = StatusSentNoReply
				result.Description = fmt.Sprintf("%s sent; no response before timeout", exchange.name)
				return nil
			}
			return err
		}
		ok, note := exchange.match(buffer[:n])
		if !ok {
			continue // stray or stale datagram; keep waiting for ours
		}
		result.Status = StatusOK
		result.Description = note
		return nil
	}
}

func genericUDPExchange() udpExchange {
	return udpExchange{
		name:    "UDP datagram",
		request: []byte("vpings\n"),
		match: func([]byte) (bool, string) {
			return true, ""
		},
	}
}

// dnsProbeName is the name the DNS probe asks for. example.com is reserved by
// IANA and always resolvable, and an ordinary A query is what resolvers and
// DNS-intercepting proxies handle best (some silently drop root NS queries).
const dnsProbeName = "example.com"

// newDNSExchange builds a recursive A query for dnsProbeName. Any reply with
// our transaction ID counts, including REFUSED, because it proves the server
// answered; a non-zero rcode is noted in the description.
func newDNSExchange() (udpExchange, error) {
	var id [2]byte
	if _, err := rand.Read(id[:]); err != nil {
		return udpExchange{}, err
	}
	request := make([]byte, 0, 12+len(dnsProbeName)+6)
	request = append(request, id[0], id[1])
	request = append(request, 0x01, 0x00) // flags: standard query, recursion desired
	request = append(request, 0x00, 0x01) // QDCOUNT
	request = append(request, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
	for _, label := range strings.Split(dnsProbeName, ".") {
		request = append(request, byte(len(label)))
		request = append(request, label...)
	}
	request = append(request, 0x00)       // end of QNAME
	request = append(request, 0x00, 0x01) // QTYPE: A
	request = append(request, 0x00, 0x01) // QCLASS: IN

	return udpExchange{
		name:    "DNS query",
		request: request,
		match: func(reply []byte) (bool, string) {
			if len(reply) < 12 || reply[0] != id[0] || reply[1] != id[1] {
				return false, ""
			}
			flags := binary.BigEndian.Uint16(reply[2:4])
			if flags&0x8000 == 0 { // not a response
				return false, ""
			}
			if rcode := flags & 0x000f; rcode != 0 {
				return true, "DNS reply rcode " + dnsRcodeName(rcode)
			}
			return true, ""
		},
	}, nil
}

func dnsRcodeName(rcode uint16) string {
	names := map[uint16]string{1: "FORMERR", 2: "SERVFAIL", 3: "NXDOMAIN", 4: "NOTIMP", 5: "REFUSED"}
	if name, ok := names[rcode]; ok {
		return name
	}
	return strconv.Itoa(int(rcode))
}

// newNTPExchange builds an NTPv4 client request. The server copies our
// transmit timestamp into the reply's origin timestamp, which identifies it.
func newNTPExchange() (udpExchange, error) {
	request := make([]byte, 48)
	request[0] = 0x23 // LI=0, VN=4, Mode=3 (client)
	if _, err := rand.Read(request[40:48]); err != nil {
		return udpExchange{}, err
	}
	token := append([]byte(nil), request[40:48]...)

	return udpExchange{
		name:    "NTP request",
		request: request,
		match: func(reply []byte) (bool, string) {
			if len(reply) < 48 || reply[0]&0x07 != 4 { // mode 4 = server
				return false, ""
			}
			if !bytes.Equal(reply[24:32], token) {
				return false, ""
			}
			if reply[1] == 0 { // stratum 0 = kiss-o'-death
				return true, "NTP kiss code " + string(bytes.TrimRight(reply[12:16], "\x00"))
			}
			return true, ""
		},
	}, nil
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
