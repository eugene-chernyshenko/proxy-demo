package proxy

import (
	"fmt"
	"io"
	"net"

	"example.com/me/myproxy/config"
	"example.com/me/myproxy/internal/logger"
	"example.com/me/myproxy/internal/plugin"
	"example.com/me/myproxy/internal/router"
	"example.com/me/myproxy/outbound"
)

// HandleConnection обрабатывает соединение от inbound и пересылает через outbound
func HandleConnection(
	inboundConn net.Conn,
	currentOutbound outbound.Outbound,
	currentOutboundID string,
	currentOutboundConfig *config.OutboundConfig,
	targetAddress string,
	inboundID string,
	rtr router.Router,
	pluginManager *plugin.Manager,
	outboundPool *outbound.Pool,
) error {
	// Создаем контекст соединения
	ctx := plugin.NewConnectionContext(inboundConn.RemoteAddr().String(), targetAddress)
	ctx.InboundID = inboundID
	ctx.OutboundID = currentOutboundID

	defer func() {
		logger.Debug("proxy", "Outbound connection to %s closed", targetAddress)
		pluginManager.OnConnectionClosed(ctx)
		inboundConn.Close()
	}()

	// Вызываем hook OnInboundConnection
	if err := pluginManager.OnInboundConnection(ctx); err != nil {
		logger.Debug("proxy", "OnInboundConnection hook error: %v", err)
		return err
	}

	// Вызываем Router для выбора outbound
	logger.Debug("proxy", "Selecting outbound for target %s", targetAddress)
	outboundID, outboundConfig, err := rtr.SelectOutbound(ctx, targetAddress, currentOutboundID, currentOutboundConfig)
	if err != nil {
		logger.Debug("proxy", "Router SelectOutbound error: %v", err)
		return err
	}

	// Определяем какой outbound использовать
	var ob outbound.Outbound
	var finalOutboundID string

	if outboundID != "" {
		// Использовать существующий outbound из пула
		if outboundPool != nil {
			poolOutbound, err := outboundPool.GetOutbound(outboundID)
			if err != nil {
				logger.Debug("proxy", "Failed to get outbound %s from pool: %v, using current", outboundID, err)
				ob = currentOutbound
				finalOutboundID = currentOutboundID
			} else {
				logger.Debug("proxy", "Router selected existing outbound %s from pool", outboundID)
				ob = poolOutbound
				finalOutboundID = outboundID
			}
		} else {
			logger.Debug("proxy", "Router selected existing outbound %s (pool not available, using current)", outboundID)
			ob = currentOutbound
			finalOutboundID = outboundID
		}
	} else if outboundConfig != nil {
		// Создать новый outbound из конфигурации
		logger.Debug("proxy", "Router selected new outbound: type=%s", outboundConfig.Type)
		ob, err = createOutbound(outboundConfig)
		if err != nil {
			logger.Debug("proxy", "Failed to create outbound: %v", err)
			return err
		}
		finalOutboundID = outboundConfig.ID
	} else {
		// Использовать текущий outbound
		logger.Debug("proxy", "Router selected current outbound")
		ob = currentOutbound
		finalOutboundID = currentOutboundID
	}

	ctx.OutboundID = finalOutboundID

	// Вызываем hook OnOutboundConnection
	if err := pluginManager.OnOutboundConnection(ctx); err != nil {
		logger.Debug("proxy", "OnOutboundConnection hook error: %v", err)
		return err
	}

	// Establish connection to target address through outbound
	logger.Debug("proxy", "Establishing outbound connection to %s", targetAddress)
	outboundConn, err := ob.Dial("tcp", targetAddress)
	if err != nil {
		logger.Debug("proxy", "Failed to connect to %s: %v", targetAddress, err)
		return err
	}
	defer outboundConn.Close()

	logger.Debug("proxy", "Outbound connection to %s established, forwarding data", targetAddress)

	// Forward data between connections with traffic counting
	err = CopyDataWithCounting(outboundConn, inboundConn, ctx, pluginManager)
	if err != nil {
		logger.Debug("proxy", "Outbound connection to %s closed with error: %v", targetAddress, err)
	} else {
		logger.Debug("proxy", "Outbound connection to %s closed normally", targetAddress)
	}
	return err
}

// createOutbound создает outbound из конфигурации
func createOutbound(cfg *config.OutboundConfig) (outbound.Outbound, error) {
	switch cfg.Type {
	case "direct":
		return outbound.NewDirectOutbound(), nil
	case "socks5":
		if cfg.ProxyAddress == "" {
			return nil, fmt.Errorf("proxy_address is required for SOCKS5 outbound")
		}
		return outbound.NewSOCKS5Outbound(cfg.ProxyAddress), nil
	default:
		return nil, fmt.Errorf("unsupported outbound type: %s", cfg.Type)
	}
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

// CopyDataWithCounting пересылает данные между двумя соединениями с подсчетом трафика
func CopyDataWithCounting(dst net.Conn, src net.Conn, ctx *plugin.ConnectionContext, pluginManager *plugin.Manager) error {
	done := make(chan error, 1)

	// Создаем обертки для подсчета байтов
	sentCounter := &countingWriter{writer: dst, ctx: ctx, pluginManager: pluginManager, direction: "sent"}
	receivedCounter := &countingWriter{writer: src, ctx: ctx, pluginManager: pluginManager, direction: "received"}

	go func() {
		_, err := io.Copy(sentCounter, src)
		done <- err
	}()

	go func() {
		_, err := io.Copy(receivedCounter, dst)
		done <- err
	}()

	// Ждем завершения одной из сторон
	err := <-done
	dst.Close()
	src.Close()
	<-done

	return err
}

// countingWriter обертка для подсчета переданных байтов
type countingWriter struct {
	writer        net.Conn
	ctx           *plugin.ConnectionContext
	pluginManager *plugin.Manager
	direction     string
	bytesWritten  int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.writer.Write(p)
	if n > 0 {
		c.bytesWritten += int64(n)
		c.pluginManager.OnDataTransfer(c.ctx, c.direction, int64(n))
	}
	return n, err
}

