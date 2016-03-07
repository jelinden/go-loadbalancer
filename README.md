# A simple load balancer / proxy

It will proxy http and websocket, using myself for proxying socket.io.

## Requirements

Balanced urls
export BALANCED_URLS=http://[192.168.0.21]:1300,http://[192.168.0.22]:1300,http://[192.168.0.23]:1300

## Build and run

```go build && ./go-loadbalancer```
