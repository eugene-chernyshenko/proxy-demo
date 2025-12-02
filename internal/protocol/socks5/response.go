package socks5

// BuildResponse строит SOCKS5 connection response
// Формат: [VER, REP, RSV, ATYP, BND.ADDR, BND.PORT]
// VER = 0x05 (SOCKS5)
// REP = reply code (0x00 = success)
// RSV = 0x00 (reserved)
// Для простоты используем 0.0.0.0:0 как BND.ADDR
func BuildResponse(reply byte) []byte {
	// [VER, REP, RSV, ATYP=IPv4, BND.ADDR=0.0.0.0, BND.PORT=0]
	return []byte{0x05, reply, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

// BuildErrorResponse строит SOCKS5 error response
func BuildErrorResponse(replyCode byte) []byte {
	return BuildResponse(replyCode)
}

// Reply codes
const (
	ReplySuccess                 = 0x00
	ReplyGeneralFailure          = 0x01
	ReplyConnectionNotAllowed    = 0x02
	ReplyNetworkUnreachable      = 0x03
	ReplyHostUnreachable         = 0x04
	ReplyConnectionRefused       = 0x05
	ReplyTTLExpired              = 0x06
	ReplyCommandNotSupported     = 0x07
	ReplyAddressTypeNotSupported = 0x08
)

