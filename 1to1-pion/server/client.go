package main

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

type Client struct {
	ID        int
	Conn      *websocket.Conn
	PC        *webrtc.PeerConnection
	AudioOut  *webrtc.TrackLocalStaticRTP
	VideoOut  *webrtc.TrackLocalStaticRTP
	clientMux sync.Mutex
	readyOnce sync.Once
	readyChan chan struct{}
}
