package peer

import (
	"github.com/oxzjh/rtc"
)

type MediaConn struct {
	connection
	*rtc.MediaConnection
}
