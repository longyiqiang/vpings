package probe

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
)

var udpPayload = []byte("vpings\n")

func probeUDP(ctx context.Context, spec Spec, result *Result) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(spec.Host, strconv.Itoa(spec.Port)))
	if err != nil {
		return err
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	}

	if _, err := conn.Write(udpPayload); err != nil {
		return err
	}

	buffer := make([]byte, 512)
	if _, err := conn.Read(buffer); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) || isTimeout(err) {
			result.Status = StatusSentNoReply
			result.Description = "UDP datagram sent; no response before timeout"
			return nil
		}
		return err
	}

	result.Status = StatusOK
	return nil
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
