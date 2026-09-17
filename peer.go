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
	sockMu    sync.Mutex
	connMu    sync.RWMutex
	conn      *websocket.Conn
	conf      *webrtc.Configuration
	conns     map[string]IConn
	onConn    func(*DataConn)
	onCall    func(*MediaConn, any) error
	onOnlines func([]uint64)
	onError   func(error)
	onClose   func()
}

func (p *Peer) start() {
	p.scheduleHeartbeat()
	go p.read()
	go p.tick()
}

func (p *Peer) scheduleHeartbeat() {
	time.AfterFunc(HeartbeatInterval, p.sendHeartbeat)
}

func (p *Peer) sendHeartbeat() {
	p.sockMu.Lock()
	err := p.conn.WriteMessage(websocket.BinaryMessage, HeartbeatMessage)
	p.sockMu.Unlock()
	if err == nil {
		p.scheduleHeartbeat()
	}
}

func (p *Peer) send(method string, data any) error {
	p.sockMu.Lock()
	defer p.sockMu.Unlock()
	return p.conn.WriteJSON(&request{method, data})
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
		if err := p.conn.ReadJSON(&resp); err != nil {
			if p.onClose != nil {
				p.onClose()
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
				if conn, err = p.newDataConn(d.Uid, d.Sign, d.Packet.ID, true); err == nil {
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

func (p *Peer) getConn(id string) IConn {
	p.connMu.RLock()
	conn := p.conns[id]
	p.connMu.RUnlock()
	return conn
}

func (p *Peer) tick() {
	for t := range time.Tick(time.Second) {
		p.connMu.Lock()
		for id, conn := range p.conns {
			if conn.isConnected() || conn.isClosed() || conn.isTimeout(t) {
				delete(p.conns, id)
			}
		}
		p.connMu.Unlock()
	}
}

func (p *Peer) newDataConn(uid uint64, sign, id string, orderd bool) (*DataConn, error) {
	dc := &DataConn{}
	dc.timeout = time.Now().Add(ConnectTimeout)
	conn, err := rtc.NewDataConnection(p.conf, func(candidate *webrtc.ICECandidateInit) {
		p.sendCandidate(uid, sign, id, candidate)
	}, orderd, dc.onData)
	if err != nil {
		return nil, err
	}
	conn.OnConnectionStateChange(dc.onConnectionStateChange)
	dc.DataConnection = conn
	p.addConn(id, dc)
	return dc, nil
}

func (p *Peer) newMediaConn(uid uint64, sign, id string) (*MediaConn, error) {
	mc := &MediaConn{}
	mc.timeout = time.Now().Add(CallTimeout)
	conn, err := rtc.NewMediaConnection(p.conf, func(candidate *webrtc.ICECandidateInit) {
		p.sendCandidate(uid, sign, id, candidate)
	})
	if err != nil {
		return nil, err
	}
	conn.OnConnectionStateChange(mc.onConnectionStateChange)
	mc.MediaConnection = conn
	p.addConn(id, mc)
	return mc, nil
}

func (p *Peer) Connect(uid uint64, sign string, ordered bool) (*DataConn, error) {
	id := uuid.NewString()
	dc, err := p.newDataConn(uid, sign, id, ordered)
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

func (p *Peer) OnConn(f func(*DataConn)) {
	p.onConn = f
}

func (p *Peer) OnCall(f func(*MediaConn, any) error) {
	p.onCall = f
}

func (p *Peer) QueryOnlines(uids []uint64) error {
	return p.send("onlines", map[string]any{"uids": uids})
}

func (p *Peer) OnOnlines(f func([]uint64)) {
	p.onOnlines = f
}

func (p *Peer) OnError(f func(error)) {
	p.onError = f
}

func (p *Peer) OnClose(f func()) {
	p.onClose = f
}

func (p *Peer) Close() error {
	return p.conn.Close()
}

func New(url string, conf *webrtc.Configuration) (*Peer, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	peer := &Peer{conn: conn, conf: conf, conns: make(map[string]IConn)}
	peer.start()
	return peer, nil
}
