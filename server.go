package tcp_sample

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	ErrEmitFmt       = "emit failed %v: %v \n"
	ErrDisconnectFmt = "disconnect listener, not found %d: %v\n"
	ClosingFmt       = "closing connection from: %v"
)

const (
	IdleTimeout          = time.Minute * 5
	MaxMessageSizeServer = 1024
)

type Server struct {
	IdleTimeout time.Duration
	tcp         net.Listener
	listeners   []net.Conn
	closed      bool
	close       chan struct{}
	sync.RWMutex
}

// ListenAndServe starts tcp server
func (s *Server) ListenAndServe(addr string) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	s.Lock()
	if s.IdleTimeout == 0 {
		s.IdleTimeout = IdleTimeout
	}
	s.tcp = l
	s.close = make(chan struct{})
	s.Unlock()

	for i := 0; ; i++ {
		conn, err := l.Accept()
		s.RLock()
		if s.closed {
			s.RUnlock()
			return nil
		}
		s.RUnlock()
		if err != nil {
			log.Println(err)
			continue
		}
		s.Lock()
		s.listeners = append(s.listeners, conn)
		s.Unlock()
		go s.listen(conn, i)
	}

	return nil
}

// Close all listeners, clean pool
func (s *Server) Close() {
	s.Lock()
	defer s.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	for _, ls := range s.listeners {
		ls.Close()
	}
	s.listeners = make([]net.Conn, 0)
}

// Shutdown stops server after listeners disconnect or by context
func (s *Server) Shutdown(ctx context.Context) {
	s.Lock()
	s.closed = true
	close(s.close)
	s.tcp.Close()
	s.Unlock()

	defer s.Close()

	log.Println("Shutdown ...")
	s.emit(nil, "Server", []byte("Server will shutdown soon \n"))

	t := time.NewTicker(time.Second)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Force shutdown")
			return
		case <-t.C:
			s.RLock()
			if 0 == len(s.listeners) {
				s.RUnlock()
				log.Println("Graceful shutdown")
				return
			}
			s.RUnlock()
		}
	}
}

// emit sends message to listeners except emitter
func (s *Server) emit(emitter net.Conn, from string, message []byte) {
	s.RLock()
	for _, ls := range s.listeners {
		if ls != emitter {
			if _, err := fmt.Fprintf(ls, "%v: %s \n", from, message); err != nil {
				log.Printf(ErrEmitFmt, from, err)
			}
		}
	}
	s.RUnlock()
}

// disconnect closes listener, remove from pool
func (s *Server) disconnect(ls net.Conn) bool {
	s.Lock()
	defer s.Unlock()
	for i, l := range s.listeners {
		if ls == l {
			ls.Close()
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			return true
		}
	}
	return false
}

// listen handle listener
func (s *Server) listen(ls net.Conn, index int) error {
	defer func() {
		log.Printf(ClosingFmt, ls.RemoteAddr())
		if removed := s.disconnect(ls); !removed {
			log.Printf(ErrDisconnectFmt, index, s)
		}
	}()

	fmt.Fprintf(ls, "Hello #%d %s \n", index, time.Now().Format(time.RFC3339))

	ls.SetDeadline(time.Now().Add(s.IdleTimeout)) // set deadline

	rl := io.LimitReader(ls, MaxMessageSizeServer)
	r := bufio.NewReader(rl)
	scanner := bufio.NewScanner(r)

	for {
		scanned := scanner.Scan()
		if !scanned {
			err := scanner.Err()
			if e, ok := err.(net.Error); ok && e.Timeout() || err == io.EOF {
				return nil
			} else if err != nil {
				log.Printf("%v - %v", err, ls.RemoteAddr())
				return err
			}
			break
		}

		ls.SetDeadline(time.Now().Add(s.IdleTimeout)) // update deadline
		go s.emit(ls, strconv.Itoa(index), scanner.Bytes())
	}

	return nil
}
