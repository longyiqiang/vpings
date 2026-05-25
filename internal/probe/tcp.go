package probe

import (
	"context"
	"net"
	"strconv"
)

func probeTCP(ctx context.Context, spec Spec) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(spec.Host, strconv.Itoa(spec.Port)))
	if err != nil {
		return err
	}
	return conn.Close()
}
