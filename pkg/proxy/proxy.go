package bpfproxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gwuah/many-ports/pkg/config"
)

type Proxy struct {
	loadbalancer   map[string]*loadbalancer
	listener       *net.TCPListener
	portToApp      map[int]string
	timeout        time.Duration
	ports          []uint16
	port           int
	maxReplayCount int
}

func New(cfg config.Config, port int) (*Proxy, error) {
	p := Proxy{
		port:           port,
		timeout:        (45 * time.Second),
		maxReplayCount: 5,
	}
	p.constructLookupTables(cfg.Apps)
	err := p.setupListeningSocket(port)
	return &p, err

}

func (p *Proxy) setupListeningSocket(port int) error {
	proxyAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%v", port))
	if err != nil {
		return err
	}

	listener, err := net.ListenTCP("tcp", proxyAddr)
	if err != nil {
		return err
	}

	p.listener = listener

	return nil
}

func (p *Proxy) constructLookupTables(apps []config.App) {
	lb := map[string]*loadbalancer{}
	portToApp := map[int]string{}
	ports := []uint16{}

	for _, app := range apps {
		lb[app.Name] = &loadbalancer{
			targets: app.Targets,
		}

		for _, port := range app.Ports {
			portToApp[port] = app.Name
			ports = append(ports, uint16(port))
		}
	}

	p.loadbalancer = lb
	p.ports = ports
	p.portToApp = portToApp

}

func (p Proxy) isTimeoutError(err error) bool {
	if opError, ok := err.(*net.OpError); ok {
		return opError.Timeout()
	}
	return false
}

func (p Proxy) getPortFromAddress(address string) (int, error) {
	addressChunks := strings.Split(address, ":")
	port := addressChunks[len(addressChunks)-1]
	return strconv.Atoi(port)
}

func (p Proxy) forwardMessage(wg *sync.WaitGroup, dst, src *net.TCPConn, direction string) {
	defer wg.Done()

	n, err := io.Copy(dst, src)
	if err != nil {
		if p.isTimeoutError(err) {
			return
		}
		log.Println("failed to forward message:", err)
	}
	log.Printf("wrote %d bytes from %s to destination %s | direction=%s", n, src.LocalAddr(), dst.RemoteAddr(), direction)
}

func (p *Proxy) GetListeningSocketFD() (uintptr, error) {
	c, err := p.listener.SyscallConn()
	if err != nil {
		return 0, err
	}
	var originalFileDescriptor uintptr
	err = c.Control(func(fd uintptr) {
		originalFileDescriptor = fd
	})
	if err != nil {
		return originalFileDescriptor, err
	}

	return originalFileDescriptor, nil
}

func (p *Proxy) GetPorts() []uint16 {
	return p.ports
}

func (p *Proxy) Proxy(ctx context.Context) {
	log.Println("proxy running on port: ", p.port)
	defer log.Println("shutting down proxy")

	defer p.listener.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := p.listener.SetDeadline(time.Now().Add(time.Second)); err != nil {
				log.Println("failed to set deadline listener: ", err)
				return
			}

			conn, err := p.listener.AcceptTCP()
			if err != nil {
				if p.isTimeoutError(err) {
					continue
				}
				log.Println("failed to accept tcp connection: ", err)
			}
			defer conn.Close()

			go p._proxy(conn)

		}
	}
}

func (p *Proxy) _proxy(originConn *net.TCPConn) {
	originConn.SetDeadline(time.Now().Add(p.timeout))
	defer originConn.Close()
	defer log.Println("proxy session ended")

	port, err := p.getPortFromAddress(originConn.LocalAddr().String())
	if err != nil {
		log.Println("failed to extract target port: ", err)
		return
	}

	replayCount := 0

replay:
	if replayCount == p.maxReplayCount {
		log.Println("see you on the other side, g. 🤟🏾: ", err)
		return
	}

	app := p.portToApp[port]
	target := p.loadbalancer[app].GetNextTartget()

	log.Printf("conn recvd from -> %s, forwarding to -> %s", originConn.LocalAddr().String(), target)

	destinationAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		log.Println("tcp address resolution failed: ", err)
		replayCount++
		goto replay
	}

	destinationConn, err := net.DialTCP("tcp", nil, destinationAddr)
	if err != nil {
		log.Println("error dialing destination server", err)
		replayCount++
		goto replay
	}
	destinationConn.SetDeadline(time.Now().Add(p.timeout))
	defer destinationConn.Close()

	wg := sync.WaitGroup{}
	wg.Add(2)

	// read from origin connection & write to destination connection
	go p.forwardMessage(&wg, destinationConn, originConn, "origin->destination")
	// read from destination connection & write back to origin connection
	go p.forwardMessage(&wg, originConn, destinationConn, "destination->origin")

	wg.Wait()
}
