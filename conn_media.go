package peer

import (
	"github.com/oxzjh/rtc"
	"github.com/pion/webrtc/v3"
)

type MediaConn struct {
	conn
	*rtc.Media
	ts []*webrtc.RTPTransceiver
}

func (c *MediaConn) AddTrack(track webrtc.TrackLocal) (*webrtc.RTPTransceiver, error) {
	if c.closed {
		return nil, ErrPeerConnectionClosed
	}
	t, err := c.Media.AddTrack(track)
	if err != nil {
		return nil, err
	}
	c.ts = append(c.ts, t)
	return t, nil
}

func (c *MediaConn) OnTrack(f func(*webrtc.TrackRemote, *webrtc.RTPReceiver)) error {
	if c.closed {
		return ErrPeerConnectionClosed
	}
	c.Media.OnTrack(f)
	return nil
}

func (c *MediaConn) Close() {
	for _, t := range c.ts {
		t.Stop()
	}
	c.Media.OnTrack(nil)
	c.conn.Close()
}
