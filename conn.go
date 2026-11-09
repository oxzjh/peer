package peer

import (
	"errors"

	"github.com/pion/webrtc/v3"
)

var ErrPeerConnectionClosed = errors.New("peer connection closed")

type IConn interface {
	CreateOffer() (*webrtc.SessionDescription, error)
	OnOffer(*webrtc.SessionDescription) (*webrtc.SessionDescription, error)
	OnAnswer(*webrtc.SessionDescription) error
	OnCandidate(*webrtc.ICECandidateInit) error
	Close()
}

type conn struct {
	closed   bool
	OnClosed func()
}

func (c *conn) Close() {
	c.closed = true
	if c.OnClosed != nil {
		c.OnClosed()
	}
}
