package peer

import (
	"errors"

	"github.com/oxzjh/rtc"
	"github.com/pion/webrtc/v3"
)

var ErrConnctionClosed = errors.New("send message with closed connection")

type DataConn struct {
	connection
	*rtc.DataConnection
	dc        *webrtc.DataChannel
	onOpen    func()
	onMessage func([]byte)
	onError   func(error)
	onClose   func()
}

func (c *DataConn) onData(dc *webrtc.DataChannel) {
	c.dc = dc
	dc.OnOpen(func() {
		if c.onOpen != nil {
			c.onOpen()
		}
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if c.onMessage != nil {
			c.onMessage(msg.Data)
		}
	})
	dc.OnError(func(err error) {
		if c.onError != nil {
			c.onError(err)
		}
	})
	dc.OnClose(func() {
		if c.onClose != nil {
			c.onClose()
		}
	})
}

func (c *DataConn) OnOpen(f func()) {
	c.onOpen = f
}

func (c *DataConn) OnMessage(f func([]byte)) {
	c.onMessage = f
}

func (c *DataConn) OnError(f func(error)) {
	c.onError = f
}

func (c *DataConn) OnClose(f func()) {
	c.onClose = f
}

func (c *DataConn) Send(data []byte) error {
	if err := c.checkState(); err != nil {
		return err
	}
	return c.dc.Send(data)
}

func (c *DataConn) SendText(text string) error {
	if err := c.checkState(); err != nil {
		return err
	}
	return c.dc.SendText(text)
}

func (c *DataConn) checkState() error {
	if c.closed {
		if c.onClose != nil {
			c.onClose()
		}
		c.dc.Close()
		return ErrConnctionClosed
	}
	return nil
}

func (c *DataConn) BufferedAmount() uint64 {
	return c.dc.BufferedAmount()
}

func (c *DataConn) Close() error {
	return c.dc.Close()
}
