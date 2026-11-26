package probe

import (
	"context"
	"net"
	"testing"
	"time"
)

// serveUDP answers each datagram with whatever reply returns (nil = stay silent).
func serveUDP(t *testing.T, reply func(request []byte) [][]byte) Spec {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	go func() {
		buffer := make([]byte, 1500)
		for {
			n, addr, err := conn.ReadFrom(buffer)
			if err != nil {
				return
			}
			for _, packet := range reply(append([]byte(nil), buffer[:n]...)) {
				conn.WriteTo(packet, addr)
			}
		}
	}()
	port := conn.LocalAddr().(*net.UDPAddr).Port
	return Spec{Protocol: ProtocolUDP, Host: "127.0.0.1", Port: port, Timeout: 300 * time.Millisecond}
}

func runExchange(t *testing.T, spec Spec, exchange udpExchange) Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), spec.Timeout)
	defer cancel()
	result := Result{Status: StatusFailed}
	if err := probeUDPExchange(ctx, spec, &result, exchange); err != nil {
		t.Fatalf("probeUDPExchange: %v", err)
	}
	return result
}

func dnsReply(request []byte, rcode byte) []byte {
	reply := append([]byte(nil), request...)
	reply[2] |= 0x80 // QR
	reply[3] = (reply[3] & 0xf0) | rcode
	return reply
}

func TestDNSExchangeMatchesOwnReply(t *testing.T) {
	spec := serveUDP(t, func(request []byte) [][]byte {
		stale := dnsReply(request, 0)
		stale[0] ^= 0xff // different transaction ID, must be ignored
		return [][]byte{stale, dnsReply(request, 0)}
	})
	exchange, err := newDNSExchange()
	if err != nil {
		t.Fatal(err)
	}
	result := runExchange(t, spec, exchange)
	if result.Status != StatusOK || result.Description != "" {
		t.Fatalf("result = %s %q, want ok", result.Status, result.Description)
	}
}

func TestDNSExchangeCountsRefusedAsReply(t *testing.T) {
	spec := serveUDP(t, func(request []byte) [][]byte { return [][]byte{dnsReply(request, 5)} })
	exchange, err := newDNSExchange()
	if err != nil {
		t.Fatal(err)
	}
	result := runExchange(t, spec, exchange)
	if result.Status != StatusOK || result.Description != "DNS reply rcode REFUSED" {
		t.Fatalf("result = %s %q, want ok with REFUSED note", result.Status, result.Description)
	}
}

func TestDNSExchangeIgnoresQueriesEchoedBack(t *testing.T) {
	// An echo server returns the query itself (QR=0); that is not an answer.
	spec := serveUDP(t, func(request []byte) [][]byte { return [][]byte{request} })
	exchange, err := newDNSExchange()
	if err != nil {
		t.Fatal(err)
	}
	result := runExchange(t, spec, exchange)
	if result.Status != StatusSentNoReply {
		t.Fatalf("status = %s, want sent_no_reply", result.Status)
	}
}

func TestNTPExchangeMatchesOriginTimestamp(t *testing.T) {
	spec := serveUDP(t, func(request []byte) [][]byte {
		reply := make([]byte, 48)
		reply[0] = 0x24 // VN=4, mode=4
		reply[1] = 2    // stratum
		copy(reply[24:32], request[40:48])
		wrong := append([]byte(nil), reply...)
		wrong[24] ^= 0xff
		return [][]byte{wrong, reply}
	})
	exchange, err := newNTPExchange()
	if err != nil {
		t.Fatal(err)
	}
	if result := runExchange(t, spec, exchange); result.Status != StatusOK {
		t.Fatalf("status = %s, want ok", result.Status)
	}
}

func TestGenericExchangeSilentServerIsSentNoReply(t *testing.T) {
	spec := serveUDP(t, func([]byte) [][]byte { return nil })
	result := runExchange(t, spec, genericUDPExchange())
	if result.Status != StatusSentNoReply {
		t.Fatalf("status = %s, want sent_no_reply", result.Status)
	}
}

func TestDNSExchangeRequestIsAQuery(t *testing.T) {
	exchange, err := newDNSExchange()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("\x07example\x03com\x00\x00\x01\x00\x01")
	if got := exchange.request[12:]; string(got) != string(want) {
		t.Fatalf("question = %q, want %q", got, want)
	}
}

func TestUDPExchangeForPort(t *testing.T) {
	for port, want := range map[int]string{53: "DNS query", 123: "NTP request", 9999: "UDP datagram"} {
		exchange, err := udpExchangeForPort(port)
		if err != nil {
			t.Fatal(err)
		}
		if exchange.name != want {
			t.Fatalf("port %d exchange = %q, want %q", port, exchange.name, want)
		}
	}
}
