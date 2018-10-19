package tcp_sample

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

const MaxMessageSizeClient = 1024

type Client struct {
	tcp    net.Conn
	closed bool
	close  chan struct{}
	sync.RWMutex
}

// Dial creates connection, starts emit and listen
func (c *Client) Dial(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	c.Lock()
	c.tcp = conn
	c.close = make(chan struct{})
	c.Unlock()

	go c.emit()
	go c.listen()

	<-c.close
}

// Close
func (c *Client) Close() {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return
	}
	c.closed = true
	close(c.close)
	c.tcp.Close()
}

// emit sends Stdin to connection
func (c *Client) emit() {
	rl := io.LimitReader(os.Stdin, MaxMessageSizeClient)
	r := bufio.NewReader(rl)
	scanner := bufio.NewScanner(r)

	for {
		scanned := scanner.Scan()
		c.RLock()
		if c.closed {
			c.RUnlock()
			return
		}
		c.RUnlock()
		if !scanned {
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
				return
			}
			break
		}

		if _, err := fmt.Fprintf(c.tcp, "%s \n", scanner.Bytes()); err != nil {
			log.Fatal(err)
		}
	}
}

// listen to connection, prints incoming messages.
// Use ReadLine to catch EOF what indicate connection was closed
func (c *Client) listen() {
	rl := io.LimitReader(c.tcp, MaxMessageSizeClient)
	r := bufio.NewReader(rl)

	for {
		line, _, err := r.ReadLine()
		if err != nil {
			c.Close()
			if e, ok := err.(net.Error); ok && e.Timeout() || err == io.EOF {
				fmt.Printf("%s \n\n", "Server closed connection")
				return
			} else if err != nil {
				log.Fatalf("%v - %v", err, c.tcp.RemoteAddr())
			}
			return
		}

		fmt.Printf("%s \n", line)
	}
}
