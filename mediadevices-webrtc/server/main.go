package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/opus"
	"github.com/pion/mediadevices/pkg/codec/x264"
	"github.com/pion/webrtc/v4"
)

type Client struct {
	ID          int
	Conn        *websocket.Conn
	PC          *webrtc.PeerConnection
	MessageChan chan []byte
}

type MessageIn struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type MessageOut struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
var pendingICECandidates = []webrtc.ICECandidateInit{}

func handleWSMessage(client *Client, msgByte []byte) {

	var msg MessageIn
	if err := json.Unmarshal(msgByte, &msg); err != nil {
		fmt.Println("message unmarshaling failed-", err)
		return
	}
	fmt.Println("handling message-", msg.Type)
	switch msg.Type {
	case "answer":
		sdp := webrtc.SessionDescription{}
		err := json.Unmarshal(msg.Data, &sdp)
		if err != nil {
			log.Printf("Error unmarshaling sdp: %v", err)
			return
		}
		if setErr := client.PC.SetRemoteDescription(sdp); setErr != nil {
			panic(setErr)
		}

		for _, candidate := range pendingICECandidates {
			if err := client.PC.AddICECandidate(candidate); err != nil {
				log.Fatal("err in adding ice candidate-", err)
				panic(err)
			}
		}
		pendingICECandidates = nil

		break
	case "ice":
		var candidate webrtc.ICECandidateInit
		err := json.Unmarshal(msg.Data, &candidate)
		if err != nil {
			log.Printf("Error unmarshaling ICE candidate: %v", err)
			return
		}
		if client.PC.RemoteDescription() != nil {
			if err := client.PC.AddICECandidate(candidate); err != nil {
				log.Fatal("err in adding ice candidate-", err)
				panic(err)
			}
		} else {
			pendingICECandidates = append(pendingICECandidates, candidate)
		}
		break
	}
}

func (c *Client) writePump() {
	for msg := range c.MessageChan {
		err := c.Conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println("Write error:", err)
			return
		}
	}

}

func handleWSConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		ID:          1,
		Conn:        conn,
		MessageChan: make(chan []byte, 32),
	}
	log.Printf("Client %d connected\n", client.ID)

	go client.writePump()

	defer func() {
		log.Printf("Client %d disconnected\n", client.ID)
		client.PC.Close()
		conn.Close()
		close(client.MessageChan)
	}()

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}
	x264Params, err := x264.NewParams()
	if err != nil {
		panic(err)
	}
	x264Params.BitRate = 500_000

	opusParams, err := opus.NewParams()
	if err != nil {
		panic(err)
	}

	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithAudioEncoders(&opusParams),
		mediadevices.WithVideoEncoders(&x264Params),
	)

	mediaEngine := webrtc.MediaEngine{}
	codecSelector.Populate(&mediaEngine)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine))

	pc, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	client.PC = pc

	streams, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(mtc *mediadevices.MediaTrackConstraints) {},
		Audio: func(mtc *mediadevices.MediaTrackConstraints) {},
		Codec: codecSelector,
	})

	if err != nil {
		fmt.Println("getusermedia err")
		panic(err)
	}

	for _, track := range streams.GetTracks() {
		track.OnEnded(func(err error) {
			fmt.Printf("Track (ID: %s) ended with error: %v\n",
				track.ID(), err)
		})

		_, err = pc.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendonly,
		})

		if err != nil {
			panic(err)
		}
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	msg, _ := json.Marshal(MessageOut{
		Type: "offer",
		Data: offer,
	})
	if err = pc.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	client.MessageChan <- msg

	pc.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}
		fmt.Println("sending ice candidate")
		candidate := i.ToJSON()
		msg, _ := json.Marshal(MessageOut{
			Type: "ice",
			Data: candidate,
		})
		client.MessageChan <- msg
	})

	pc.OnSignalingStateChange(func(ss webrtc.SignalingState) {
		fmt.Println("signaling state changed:", ss)
	})

	pc.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		fmt.Println("peer connection state changed:", pcs)
	})

	pc.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
		fmt.Println("ice connection state: ", is)
	})

	for {
		_, msgByte, err := conn.ReadMessage()
		fmt.Println("incoming message")
		if err != nil {
			return
		}
		handleWSMessage(client, msgByte)
	}
}

func main() {

	http.HandleFunc("/ws", handleWSConnection)
	fmt.Println("Server started")
	log.Fatal(http.ListenAndServe(":9091", nil))

}
