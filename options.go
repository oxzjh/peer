package peer

type Option func(*Peer)

func WithOnOnlines(onOnlines func([]uint64)) Option {
	return func(p *Peer) {
		p.onOnlines = onOnlines
	}
}

func WithOnError(onError func(error)) Option {
	return func(p *Peer) {
		p.onError = onError
	}
}

func WithOnClosed(onClosed func()) Option {
	return func(p *Peer) {
		p.onClosed = onClosed
	}
}

func WithOnConn(onConn func(*DataConn)) Option {
	return func(p *Peer) {
		p.onConn = onConn
	}
}

func WithOnCall(onCall func(*MediaConn, any) error) Option {
	return func(p *Peer) {
		p.onCall = onCall
	}
}
