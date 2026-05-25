package probe

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"

	"github.com/quic-go/quic-go"
)

func probeQUIC(ctx context.Context, spec Spec) error {
	addr := net.JoinHostPort(spec.Host, strconv.Itoa(spec.Port))
	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		ServerName:         spec.Host,
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3", "hq-29"},
	}, &quic.Config{
		HandshakeIdleTimeout: spec.Timeout,
		MaxIdleTimeout:       spec.Timeout,
	})
	if err != nil {
		return err
	}
	return conn.CloseWithError(0, "vpings probe")
}
