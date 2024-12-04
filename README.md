# ManyPorts
This is a simple tcp proxy that's powered by ebpf router running in the kernel.

# How it works
The application has 2 parts. 
- ebpf (kernel) - routes all network traffic to multiple ports to a single socket.
- proxy (userspace) - directs traffic to the appropriate backend.
See config.json for how the routing map looks like.

## How to regenerate bpf files
- Run `make deps` to install bpf2go tool.
- run `make generate` to generate the object files & go types.

## How to run
After generating all the bpf files, run `go run cmd/proxy/*.go`

## Versions
- Ubuntu, 20.04.4
- Clang, 10.0.0
- Golang, 1.18 (linux/amd64)
