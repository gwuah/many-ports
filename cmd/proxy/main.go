package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/gwuah/many-ports/pkg/config"
	bpfproxy "github.com/gwuah/many-ports/pkg/proxy"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc $BPF_CLANG -cflags $BPF_CFLAGS bpf steer.c -- -I../headers

func main() {
	ctx := newCancelableContext()

	cfgStore := config.NewConfigStore("./config.json")
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal(err)
	}

	// load bpf objects into kernel
	objs := bpfObjects{}
	if err := loadBpfObjects(&objs, nil); err != nil {
		log.Fatalf("loading objects: %v", err)
	}
	defer objs.Close()

	// get the network namespace of the program
	netns, err := os.Open("/proc/self/ns/net")
	if err != nil {
		log.Fatalf("error loading namespace: %v", err)
	}
	defer netns.Close()

	// attach the bpf program to the network namespace
	link, err := link.AttachNetNs(int(netns.Fd()), objs.Steer)
	if err != nil {
		log.Fatalf("error attaching descriptor to namespace: %v", err)
	}
	defer link.Close()

	cfg, err := cfgStore.Read()
	if err != nil {
		log.Fatalf("failed to read config. err=%v", err)
	}

	p, err := bpfproxy.New(cfg, 8080)
	if err != nil {
		log.Fatalf("failed to initialize bpf layered proxy: %v", err)
	}

	ports := p.GetPorts()
	// insert all ports into the bpf map
	n, err := objs.bpfMaps.Ports.BatchUpdate(ports, make([]uint8, len(ports)), nil)
	if err != nil {
		log.Fatalf("failed to seed ports map: %v", err)
	}

	if len(ports) != n {
		log.Fatalf("failed to seed ports map, total writes -> %v", n)
	}

	fd, err := p.GetListeningSocketFD()
	if err != nil {
		log.Fatalf("failed to retreive fd of proxy socket: %v", err)
	}

	// insert all the file descriptor to the our listener socket into the dedicated socket map
	err = objs.bpfMaps.DedicatedSocket.Put(uint32(0), uint64(fd))
	if err != nil {
		log.Fatalf("failed to seed dedicated socket map: %v", err)
	}

	go p.Proxy(ctx)

	<-ctx.Done()
}

// newCancelableContext returns a context that gets canceled by a SIGINT
func newCancelableContext() context.Context {
	doneCh := make(chan os.Signal, 1)
	signal.Notify(doneCh, os.Interrupt)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-doneCh
		log.Println("signal recieved")
		cancel()
	}()

	return ctx
}
