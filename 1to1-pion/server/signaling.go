package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

func (s *Server) handleSignal(c *Client, raw []byte) {
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		fmt.Println("message unmarshaling failed-", err)
		return
	}

	fmt.Println("handling message-", msg.Type)

	switch msg.Type {

	case "offer":
		log.Println("offer received")
		var offer webrtc.SessionDescription
		if err := json.Unmarshal(msg.Data, &offer); err != nil {
			log.Println("failed to unmarshal offer:", err)
			return
		}
		if setErr := c.PC.SetRemoteDescription(offer); setErr != nil {
			panic(setErr)
		}

		answer, err := c.PC.CreateAnswer(nil)
	
		if err != nil {
			log.Fatalln("err in creating answer-", err)
		}
		if setErr := c.PC.SetLocalDescription(answer); setErr != nil {
			panic(setErr)
		}

		ansMesg := MessageOut{
			Type: "answer",
			Data: map[string]string{
				"type": "answer",
				"sdp":  answer.SDP,
			},
		}

		ansMesgBytes, _ := json.Marshal(ansMesg)

		fmt.Println("sending answer message")
		c.clientMux.Lock()
		defer c.clientMux.Unlock()
		c.Conn.WriteMessage(websocket.TextMessage, ansMesgBytes)

	case "ice":
		var candidate webrtc.ICECandidateInit
		err := json.Unmarshal(msg.Data, &candidate)
		if err != nil {
			log.Printf("Error unmarshaling ICE candidate: %v", err)
			return
		}
		if err := c.PC.AddICECandidate(candidate); err != nil {
			log.Fatal("err in adding ice candidate-", err)
			panic(err)
		}
	}
}
