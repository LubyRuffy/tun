package proxy

import (
	"errors"
	"net"

	"github.com/4396/tun/dialer"
	"github.com/4396/tun/traffic"
	"github.com/golang/sync/syncmap"
)

type Service struct {
	Traff   traffic.Traffic
	proxies syncmap.Map
	proxyc  chan Proxy
	connc   chan proxyConn
	donec   chan interface{}
	errc    chan error
}

type proxyConn struct {
	Proxy
	net.Conn
}

func (s *Service) Shutdown() (err error) {
	if s.donec != nil {
		close(s.donec)
		s.donec = nil
	}
	return
}

func (s *Service) Serve() (err error) {
	s.errc = make(chan error, 1)
	s.donec = make(chan interface{})
	s.proxyc = make(chan Proxy, 16)
	s.connc = make(chan proxyConn, 16)

	s.proxies.Range(func(key, val interface{}) bool {
		go s.listenProxy(val.(Proxy))
		return true
	})

	for {
		select {
		case p := <-s.proxyc:
			go s.listenProxy(p)
		case c := <-s.connc:
			go s.handleConn(c)
		case err = <-s.errc:
			s.Shutdown()
			return
		case <-s.donec:
			return
		}
	}
}

func (s *Service) Proxy(p Proxy) (err error) {
	_, loaded := s.proxies.LoadOrStore(p.Name(), p)
	if loaded {
		err = errors.New("Already existed")
		return
	}

	if s.proxyc != nil {
		s.proxyc <- p
	}
	return
}

func (s *Service) Proxies() (proxies []Proxy) {
	s.proxies.Range(func(key, val interface{}) bool {
		proxies = append(proxies, val.(Proxy))
		return true
	})
	return
}

var ErrInvalidProxy = errors.New("Invalid proxy")

func (s *Service) Register(name string, dialer dialer.Dialer) (err error) {
	if val, ok := s.proxies.Load(name); ok {
		err = val.(Proxy).Bind(dialer)
	} else {
		err = ErrInvalidProxy
	}
	return
}

func (s *Service) Unregister(name string, dialer dialer.Dialer) (err error) {
	if val, ok := s.proxies.Load(name); ok {
		err = val.(Proxy).Unbind(dialer)
	} else {
		err = ErrInvalidProxy
	}
	return
}

func (s *Service) listenProxy(p Proxy) {
	defer p.Close()
	for {
		conn, err := p.Accept()
		if err != nil {
			s.errc <- err
			return
		}

		select {
		case <-s.donec:
			return
		default:
			s.connc <- proxyConn{p, conn}
		}
	}
}

func (s *Service) handleConn(pc proxyConn) {
	err := pc.Proxy.Handle(pc.Conn, s.Traff)
	if err != nil {
		// ...
	}
}