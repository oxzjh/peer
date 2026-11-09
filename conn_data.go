package peer

import (
	"github.com/oxzjh/rtc"
	"github.com/pion/webrtc/v3"
)

type DataConn struct {
	conn
	*rtc.Data
	dc        *webrtc.DataChannel
	OnOpen    func()
	OnMessage func([]byte)
	OnError   func(error)
	OnClose   func()
}

func (c *DataConn) onData(dc *webrtc.DataChannel) {
	c.dc = dc
	dc.OnOpen(func() {
		if c.OnOpen != nil {
			c.OnOpen()
		}
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if c.OnMessage != nil {
			c.OnMessage(msg.Data)
		}
	})
	dc.OnError(func(err error) {
		if c.OnError != nil {
			c.OnError(err)
		}
	})
	dc.OnClose(func() {
		if c.OnClose != nil {
			c.OnClose()
		}
	})
}

func (c *DataConn) Send(data []byte) error {
	if c.closed {
		return ErrPeerConnectionClosed
	}
	return c.dc.Send(data)
}

func (c *DataConn) SendText(text string) error {
	if c.closed {
		return ErrPeerConnectionClosed
	}
	return c.dc.SendText(text)
}

func (c *DataConn) BufferedAmount() uint64 {
	return c.dc.BufferedAmount()
}

func (c *DataConn) Close() {
	if c.dc != nil {
		c.dc.Close()
	}
	c.conn.Close()
}
