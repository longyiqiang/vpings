package probe

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const icmpProtocolNumber = 1

func probeICMP(ctx context.Context, spec Spec) error {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", spec.Host)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return fmt.Errorf("no IPv4 address found for %s", spec.Host)
	}

	var firstErr error
	for _, network := range []string{"udp4", "ip4:icmp"} {
		if err := probeICMPWithNetwork(ctx, network, ips[0]); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		return nil
	}
	return firstErr
}

func probeICMPWithNetwork(ctx context.Context, network string, ip net.IP) error {
	conn, err := icmp.ListenPacket(network, "0.0.0.0")
	if err != nil {
		return err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	}

	id := os.Getpid() & 0xffff
	seq := int(time.Now().UnixNano() & 0xffff)
	message := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: []byte("vpings"),
		},
	}
	data, err := message.Marshal(nil)
	if err != nil {
		return err
	}

	if _, err := conn.WriteTo(data, icmpDestination(network, ip)); err != nil {
		return err
	}

	buffer := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			return err
		}
		reply, err := icmp.ParseMessage(icmpProtocolNumber, buffer[:n])
		if err != nil {
			continue
		}
		if reply.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		echo, ok := reply.Body.(*icmp.Echo)
		if !ok {
			continue
		}
		if echo.ID == id && echo.Seq == seq {
			return nil
		}
	}
}

func icmpDestination(network string, ip net.IP) net.Addr {
	if network == "udp4" {
		return &net.UDPAddr{IP: ip}
	}
	return &net.IPAddr{IP: ip}
}
