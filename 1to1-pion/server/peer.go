package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

func NewPeer(client *Client, room *Room) (*webrtc.PeerConnection, error) {

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}

	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"sfu",
	)
	if err != nil {
		return nil, err
	}

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video",
		"sfu",
	)
	if err != nil {
		return nil, err
	}

	if _, err := pc.AddTrack(audioTrack); err != nil {
		return nil, err
	}

	if _, err := pc.AddTrack(videoTrack); err != nil {
		return nil, err
	}

	client.AudioOut = audioTrack
	client.VideoOut = videoTrack

	client.AudioSwitcher = NewMediaSwitcher(audioTrack)
	client.VideoSwitcher = NewMediaSwitcher(videoTrack)

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

	pc.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
		log.Println("ICE state:", is.String())

		if is == webrtc.ICEConnectionStateCompleted || is == webrtc.ICEConnectionStateConnected {
			client.readyOnce.Do(func() {
				close(client.readyChan)
				log.Println("client", client.ID, "is READY")
			})
		}
	})

	pc.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		log.Printf("Track recieved: kind=%s, codec=%s", tr.Kind(), tr.Codec().MimeType)

		// ---------- Direction B: client1 → client2 ----------
		if client.ID == 1 {
			fmt.Println("processing client1")
			peer := room.GetClientById(2)
			if peer == nil {
				return
			}

			<-peer.readyChan

			var out *webrtc.TrackLocalStaticRTP
			if tr.Kind() == webrtc.RTPCodecTypeVideo {
				out = peer.VideoOut
			} else {
				out = peer.AudioOut
			}

			go func() {
				for {
					pkt, _, err := tr.ReadRTP()
					if err != nil {
						return
					}
					if err := out.WriteRTP(pkt); err != nil {
						log.Println("RTP write error:", err)
						return
					}
				}
			}()
			return
		}

		// ---------- Direction A: client2/client3 → client1 ----------
		if client.ID != 2 && client.ID != 3 {
			fmt.Println("returning")
			return
		}

		fmt.Println("processing client2|3")
		var peer = room.GetClientById(1)

		if peer == nil {
			log.Println("no peer to forward to")
			return
		}
		log.Println("fetch other peer-", peer.ID)
		if peer.PC == nil {
			fmt.Println("other peer pc nil")
		}
		log.Println("waiting for other peer to become ready:", peer.ID)

		<-peer.readyChan
		fmt.Println(peer.PC.ConnectionState())
		fmt.Println(peer.PC.ICEConnectionState())
		log.Println("other peer is ready, start forwarding to:", peer.ID)

		var switcher *MediaSwitcher
		if tr.Kind() == webrtc.RTPCodecTypeAudio {
			switcher = peer.AudioSwitcher
		} else {
			switcher = peer.VideoSwitcher
		}
		fmt.Println("clientID=", client.ID)

		switcher.SwitchTo(client.ID, peer.PC, tr)
		fmt.Println("switcher done")

		for {
			pkt, _, err := tr.ReadRTP()
			if err != nil {
				log.Println("RTP read error:", err)
				return
			}

			if switcher.activeSource == client.ID {
				switcher.packetChan <- pkt
			}
		}
	})
	return pc, nil
}
