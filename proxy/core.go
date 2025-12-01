package proxy

import (
	"io"
	"net"

	"example.com/me/myproxy/internal/logger"
)

// HandleConnection обрабатывает соединение от inbound и пересылает через outbound
func HandleConnection(inboundConn net.Conn, dial func(network, address string) (net.Conn, error), targetAddress string) error {
	defer func() {
		logger.Debug("proxy", "Proxy connection to %s closed", targetAddress)
		inboundConn.Close()
	}()

	// Establish connection to target address through outbound
	logger.Debug("proxy", "Establishing proxy connection to %s", targetAddress)
	outboundConn, err := dial("tcp", targetAddress)
	if err != nil {
		logger.Debug("proxy", "Failed to connect to %s: %v", targetAddress, err)
		return err
	}
	defer outboundConn.Close()

	logger.Debug("proxy", "Proxy connection to %s established, forwarding data", targetAddress)

	// Forward data between connections
	err = CopyData(outboundConn, inboundConn)
	if err != nil {
		logger.Debug("proxy", "Proxy connection to %s closed with error: %v", targetAddress, err)
	} else {
		logger.Debug("proxy", "Proxy connection to %s closed normally", targetAddress)
	}
	return err
}

// CopyData пересылает данные между двумя соединениями
func CopyData(dst net.Conn, src net.Conn) error {
	done := make(chan error, 1)

	go func() {
		_, err := io.Copy(dst, src)
		done <- err
	}()

	go func() {
		_, err := io.Copy(src, dst)
		done <- err
	}()

	// Ждем завершения одной из сторон
	err := <-done
	dst.Close()
	src.Close()
	<-done

	return err
}

