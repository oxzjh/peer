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
}
