# A simple load balancer / proxy

It will proxy http and websocket, using myself for proxying socket.io.

## Requirements

Balanced urls
export BALANCED_URLS=192.168.0.21,192.168.0.22,192.168.0.23

## Build and run

```go build && ./go-loadbalancer```
