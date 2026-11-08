package peer

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/oxzjh/rtc"
	"github.com/pion/webrtc/v3"
)

var (
	HeartbeatInterval = time.Minute
	HeartbeatMessage  = []byte("{\"method\":\"heartbeat\"}")
	ConnectTimeout    = 30 * time.Second
	ConnectOrdered    = true
	CallTimeout       = time.Minute
)

type request struct {
	Method string `json:"method"`
	Data   any    `json:"data,omitempty"`
}

type response struct {
	Method string          `json:"method"`
	Data   json.RawMessage `json:"data"`
}

type data struct {
	Uid    uint64  `json:"uid"`
	Sign   string  `json:"sign"`
	Packet *packet `json:"packet"`
}

type packet struct {
	ID        string                     `json:"id"`
	Type      string                     `json:"type"`
	Media     any                        `json:"media,omitempty"`
	SDP       *webrtc.SessionDescription `json:"sdp,omitempty"`
	Candidate *webrtc.ICECandidateInit   `json:"candidate,omitempty"`
}

type Peer struct {
	conf      *webrtc.Configuration
	ws        *websocket.Conn
	wsMu      sync.Mutex
	conns     map[string]IConn
	connMu    sync.RWMutex
	onOnlines func([]uint64)
	onError   func(error)
	onClosed  func()
	onConn    func(*DataConn)
	onCall    func(*MediaConn, any) error
}

func (p *Peer) start() {
	p.scheduleHeartbeat()
	go p.read()
}

func (p *Peer) scheduleHeartbeat() {
	time.AfterFunc(HeartbeatInterval, p.sendHeartbeat)
}

func (p *Peer) sendHeartbeat() {
	p.wsMu.Lock()
	err := p.ws.WriteMessage(websocket.BinaryMessage, HeartbeatMessage)
	p.wsMu.Unlock()
	if err == nil {
		p.scheduleHeartbeat()
	}
}

func (p *Peer) send(method string, data any) error {
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	return p.ws.WriteJSON(&request{method, data})
}

func (p *Peer) sendto(uid uint64, sign string, packet *packet) error {
	return p.send("sendto", &data{uid, sign, packet})
}

func (p *Peer) sendCandidate(uid uint64, sign, id string, candidate *webrtc.ICECandidateInit) error {
	return p.sendto(uid, sign, &packet{
		ID:        id,
		Type:      "candidate",
		Candidate: candidate,
	})
}

func (p *Peer) sendOffer(c IConn, uid uint64, sign, id string, media any) error {
	offer, err := c.CreateOffer()
	if err != nil {
		return err
	}
	return p.sendto(uid, sign, &packet{
		ID:    id,
		Type:  "offer",
		Media: media,
		SDP:   offer,
	})
}

func (p *Peer) onOffer(c IConn, uid uint64, sign, id string, offer *webrtc.SessionDescription) error {
	answer, err := c.OnOffer(offer)
	if err != nil {
		return err
	}
	return p.sendto(uid, sign, &packet{
		ID:   id,
		Type: "answer",
		SDP:  answer,
	})
}

func (p *Peer) read() {
	for {
		var resp response
		if err := p.ws.ReadJSON(&resp); err != nil {
			if p.onClosed != nil {
				p.onClosed()
			}
			return
		}
		switch resp.Method {
		case "recvfrom":
			var d data
			json.Unmarshal(resp.Data, &d)
			if d.Packet != nil {
				p.onRecvfrom(&d)
			}
		case "onlines":
			if p.onOnlines != nil {
				var uids []uint64
				json.Unmarshal(resp.Data, &uids)
				p.onOnlines(uids)
			}
		}
	}
}

func (p *Peer) onRecvfrom(d *data) {
	var err error
	switch d.Packet.Type {
	case "offer":
		if d.Packet.Media == nil {
			if p.onConn != nil {
				var conn *DataConn
				if conn, err = p.newDataConn(d.Uid, d.Sign, d.Packet.ID); err == nil {
					p.onConn(conn)
					err = p.onOffer(conn, d.Uid, d.Sign, d.Packet.ID, d.Packet.SDP)
				}
			}
		} else if p.onCall != nil {
			var conn *MediaConn
			if conn, err = p.newMediaConn(d.Uid, d.Sign, d.Packet.ID); err == nil {
				if err = p.onCall(conn, d.Packet.Media); err == nil {
					err = p.onOffer(conn, d.Uid, d.Sign, d.Packet.ID, d.Packet.SDP)
				}
			}
		}
	case "answer":
		if conn := p.getConn(d.Packet.ID); conn != nil {
			err = conn.OnAnswer(d.Packet.SDP)
		}
	case "candidate":
		if conn := p.getConn(d.Packet.ID); conn != nil {
			err = conn.OnCandidate(d.Packet.Candidate)
		}
	}
	if err != nil && p.onError != nil {
		p.onError(err)
	}
}

func (p *Peer) addConn(id string, conn IConn) {
	p.connMu.Lock()
	p.conns[id] = conn
	p.connMu.Unlock()
}

func (p *Peer) delConn(id string) {
	p.connMu.Lock()
	delete(p.conns, id)
	p.connMu.Unlock()
}

func (p *Peer) getConn(id string) IConn {
	p.connMu.RLock()
	conn := p.conns[id]
	p.connMu.RUnlock()
	return conn
}

func (p *Peer) newDataConn(uid uint64, sign, id string) (*DataConn, error) {
	dc := &DataConn{}
	d, err := rtc.NewData(p.conf, func(candidate *webrtc.ICECandidateInit) {
		p.sendCandidate(uid, sign, id, candidate)
	}, dc.onData, rtc.WithOrdered(ConnectOrdered), rtc.WithTimeout(ConnectTimeout), rtc.WithOnTimeout(func() {
		p.delConn(id)
	}), rtc.WithOnClosed(func() {
		dc.closed = true
		p.delConn(id)
	}))
	if err != nil {
		return nil, err
	}
	dc.Data = d
	p.addConn(id, dc)
	return dc, nil
}

func (p *Peer) newMediaConn(uid uint64, sign, id string) (*MediaConn, error) {
	mc := &MediaConn{}
	m, err := rtc.NewMedia(p.conf, func(candidate *webrtc.ICECandidateInit) {
		p.sendCandidate(uid, sign, id, candidate)
	}, rtc.WithTimeout(CallTimeout), rtc.WithOnTimeout(func() {
		p.delConn(id)
	}), rtc.WithOnClosed(func() {
		p.delConn(id)
	}))
	if err != nil {
		return nil, err
	}
	mc.Media = m
	p.addConn(id, mc)
	return mc, nil
}

func (p *Peer) Connect(uid uint64, sign string) (*DataConn, error) {
	id := uuid.NewString()
	dc, err := p.newDataConn(uid, sign, id)
	if err != nil {
		return nil, err
	}
	if err = p.sendOffer(dc, uid, sign, id, nil); err != nil {
		dc.Close()
		return nil, err
	}
	return dc, nil
}

func (p *Peer) Call(uid uint64, sign string, media any) (*MediaConn, error) {
	if media == nil {
		return nil, errors.New("call without media")
	}
	id := uuid.NewString()
	mc, err := p.newMediaConn(uid, sign, id)
	if err != nil {
		return nil, err
	}
	if err = p.sendOffer(mc, uid, sign, id, media); err != nil {
		mc.Close()
		return nil, err
	}
	return mc, nil
}

func (p *Peer) QueryOnlines(uids []uint64) error {
	return p.send("onlines", map[string]any{"uids": uids})
}

func (p *Peer) Close() error {
	return p.ws.Close()
}

func New(url string, conf *webrtc.Configuration, opts ...Option) (*Peer, error) {
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	p := &Peer{conf: conf, ws: ws, conns: make(map[string]IConn)}
	for _, o := range opts {
		o(p)
	}
	p.start()
	return p, nil
}
