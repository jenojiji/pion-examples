package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
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

		other := room.Other(client.ID)
		if other == nil {
			log.Println("no peer to forward to")
			return
		}
		log.Println("fetch other peer-", other.ID)
		if other.PC == nil {
			fmt.Println("other peer pc nil")
		}
		log.Println("waiting for other peer to become ready:", other.ID)

		<-other.readyChan
		fmt.Println(other.PC.ConnectionState())
		fmt.Println(other.PC.ICEConnectionState())

		log.Println("other peer is ready, start forwarding to:", other.ID)

		var outTrack *webrtc.TrackLocalStaticRTP
		if tr.Kind() == webrtc.RTPCodecTypeAudio {
			outTrack = other.AudioOut
		} else if tr.Kind() == webrtc.RTPCodecTypeVideo {
			outTrack = other.VideoOut
		}
		log.Println(outTrack)

		if tr.Kind() == webrtc.RTPCodecTypeVideo {
			go func() {
				log.Println("sending PLI to request keyframe")
				_ = client.PC.WriteRTCP([]rtcp.Packet{
					&rtcp.PictureLossIndication{
						MediaSSRC: uint32(tr.SSRC()),
					},
				})
			}()
		}

		go func() {
			// buf := make([]byte, 1400)
			// rtpPacket := &rtp.Packet{}
			for {
				pkt, _, err := tr.ReadRTP()
				if err != nil {
					log.Println("RTP read error:", err)
					return
				}
				// log.Println("packet read")

				if err := outTrack.WriteRTP(pkt); err != nil {
					log.Println("RTP write error:", err)
					return
				}
				// log.Println("packet write")
			}
		}()

	})
	return pc, nil
}
