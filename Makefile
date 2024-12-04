
CLANG ?= clang
CFLAGS := -O2 -g -Wall -Werror $(CFLAGS)

export BPF_CFLAGS := $(CFLAGS)
export BPF_CLANG := $(CLANG)

deps:
	go get github.com/cilium/ebpf/cmd/bpf2go

generate:
	cd cmd/proxy && go generate ./... 
	cd cmd/proxy && rm -rf bpf_bpfeb.go && rm -rf bpf_bpfeb.o