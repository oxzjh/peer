package peer

import (
	"github.com/pion/webrtc/v3"
)

type IConn interface {
	CreateOffer() (*webrtc.SessionDescription, error)
	OnOffer(*webrtc.SessionDescription) (*webrtc.SessionDescription, error)
	OnAnswer(*webrtc.SessionDescription) error
	OnCandidate(*webrtc.ICECandidateInit) error
	Close() error
	// isConnected() bool
	// isClosed() bool
	// isTimeout(time.Time) bool
}

// type connection struct {
// 	connected bool
// 	closed    bool
// 	timeout   time.Time
// 	onTimeout func()
// }

// func (c *connection) isConnected() bool {
// 	return c.connected
// }

// func (c *connection) isClosed() bool {
// 	return c.closed
// }

// func (c *connection) isTimeout(t time.Time) bool {
// 	timeout := t.After(c.timeout)
// 	if timeout {
// 		c.closed = true
// 		if c.onTimeout != nil {
// 			c.onTimeout()
// 		}
// 	}
// 	return timeout
// }

// func (c *connection) onConnectionStateChange(state webrtc.PeerConnectionState) {
// 	switch state {
// 	case webrtc.PeerConnectionStateConnected:
// 		c.connected = true
// 	case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
// 		c.closed = true
// 	}
// }

// func (c *connection) OnTimeout(f func()) {
// 	c.onTimeout = f
// }
