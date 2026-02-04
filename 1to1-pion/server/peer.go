package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func NewPeer(client *Client, room *Room) (*webrtc.PeerConnection, error) {
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}

	_, err = pc.AddTransceiverFromKind(
		webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendrecv,
		},
	)
	if err != nil {
		return nil, err
	}

	_, err = pc.AddTransceiverFromKind(
		webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendrecv,
		},
	)
	if err != nil {
		return nil, err
	}

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		fmt.Println("sending ice candidate")
		candidate := c.ToJSON()
		msg, _ := json.Marshal(MessageOut{
			Type: "ice",
			Data: candidate,
		})
		client.clientMux.Lock()
		defer client.clientMux.Unlock()
		client.Conn.WriteMessage(websocket.TextMessage, msg)

	})

	pc.OnSignalingStateChange(func(s webrtc.SignalingState) {
		fmt.Println("signaling state changed:", s)
	})

	pc.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		fmt.Println("peer connection state changed:", pcs)
	})

	pc.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		log.Printf("Track recieved: kind=%s, codec=%s", tr.Kind(), tr.Codec().MimeType)
		go func() {
			buf := make([]byte, 1400)
			rtpPacket := &rtp.Packet{}
			for {
				_, _, readErr := tr.Read(buf)
				if readErr != nil {
					log.Println("Track read error:", readErr)
					return
				}
				// In a real application, you would forward the RTP packets to another peer or process them.

				fmt.Printf(rtpPacket.String())
				// fmt.Println(buf[:n])
			}
		}()

	})
	return pc, nil
}
