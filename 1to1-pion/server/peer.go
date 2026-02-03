package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

func NewPeer(client *Client, room *Room) (*webrtc.PeerConnection, error) {
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		fmt.Println("sending ice candidate")
		msg, _ := json.Marshal(Message{
			Type: "ice",
			Data: json.RawMessage(strconv.Quote(c.ToJSON().Candidate)),
		})
		client.clientMux.Lock()
		defer client.clientMux.Unlock()
		client.Conn.WriteMessage(websocket.TextMessage, msg)

	})
	return pc, nil
}
