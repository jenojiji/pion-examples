package main

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/x264"
	_ "github.com/pion/mediadevices/pkg/driver/camera"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v4"
)

/* ----------------------------- Types ----------------------------- */

type Client struct {
	ID          int
	Conn        *websocket.Conn
	PC          *webrtc.PeerConnection
	MessageChan chan []byte

	VideoSender *webrtc.RTPSender
	VideoStream mediadevices.MediaStream
}

type MessageIn struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type MessageOut struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

/* ----------------------------- Globals ----------------------------- */

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var pendingICECandidates []webrtc.ICECandidateInit

var (
	video3DID string
	video4DID string
)

/* ----------------------------- Media Helpers ----------------------------- */

func getVideoStream(deviceID string, codecSelector *mediadevices.CodecSelector) (mediadevices.MediaStream, error) {
	return mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(mtc *mediadevices.MediaTrackConstraints) {
			mtc.DeviceID = prop.String(deviceID)
			mtc.Width = prop.Int(1280)
			mtc.Height = prop.Int(720)
			mtc.FrameRate = prop.Float(30)
		},
		Codec: codecSelector,
	})
}

func closeStream(stream mediadevices.MediaStream) {
	for _, t := range stream.GetTracks() {
		_ = t.Close()
	}
}

func (c *Client) switchVideo(deviceID string, codecSelector *mediadevices.CodecSelector) error {
	log.Println("Switching video to:", deviceID)

	if c.VideoStream != nil {
		closeStream(c.VideoStream)
	}

	stream, err := getVideoStream(deviceID, codecSelector)
	if err != nil {
		return err
	}

	newTrack := stream.GetVideoTracks()[0]

	if err := c.VideoSender.ReplaceTrack(newTrack); err != nil {
		return err
	}

	c.VideoStream = stream
	log.Println("Video track replaced successfully")
	return nil
}

/* ----------------------------- WebSocket ----------------------------- */

func handleWSMessage(client *Client, msgByte []byte, codecSelector *mediadevices.CodecSelector) {
	var msg MessageIn
	if err := json.Unmarshal(msgByte, &msg); err != nil {
		log.Println("WS unmarshal failed:", err)
		return
	}

	switch msg.Type {

	case "answer":
		var sdp webrtc.SessionDescription
		_ = json.Unmarshal(msg.Data, &sdp)
		_ = client.PC.SetRemoteDescription(sdp)

		for _, c := range pendingICECandidates {
			_ = client.PC.AddICECandidate(c)
		}
		pendingICECandidates = nil

	case "ice":
		var c webrtc.ICECandidateInit
		_ = json.Unmarshal(msg.Data, &c)

		if client.PC.RemoteDescription() != nil {
			_ = client.PC.AddICECandidate(c)
		} else {
			pendingICECandidates = append(pendingICECandidates, c)
		}

	case "pixelationLevelChange":
		var level int
		if err := json.Unmarshal(msg.Data, &level); err != nil {
			var s string
			_ = json.Unmarshal(msg.Data, &s)
			level, _ = strconv.Atoi(s)
		}

		addr := net.UnixAddr{Name: "/tmp/pixel_block.sock", Net: "unixgram"}
		conn, err := net.DialUnix("unixgram", nil, &addr)
		if err != nil {
			return
		}
		defer conn.Close()

		_ = binary.Write(conn, binary.LittleEndian, int32(level))

	case "pixelate":
		log.Println("Pixelate → video4")
		_ = client.switchVideo(video4DID, codecSelector)

	case "unpixelate":
		log.Println("Unpixelate → video3")
		_ = client.switchVideo(video3DID, codecSelector)
	}
}

func (c *Client) writePump() {
	for msg := range c.MessageChan {
		_ = c.Conn.WriteMessage(websocket.TextMessage, msg)
	}
}

/* ----------------------------- HTTP Handler ----------------------------- */

func handleWSConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		ID:          1,
		Conn:        conn,
		MessageChan: make(chan []byte, 16),
	}

	go client.writePump()
	defer func() {
		if client.VideoStream != nil {
			closeStream(client.VideoStream)
		}
		if client.PC != nil {
			_ = client.PC.Close()
		}
		conn.Close()
	}()

	/* -------------------- WebRTC Setup -------------------- */

	x264Params, _ := x264.NewParams()
	x264Params.BitRate = 1_500_000

	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithVideoEncoders(&x264Params),
	)

	mediaEngine := webrtc.MediaEngine{}
	codecSelector.Populate(&mediaEngine)

	api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine))
	pc, _ := api.NewPeerConnection(webrtc.Configuration{})
	client.PC = pc

	/* -------------------- Devices -------------------- */

	for _, d := range mediadevices.EnumerateDevices() {
		if d.Kind == mediadevices.VideoInput {
			if strings.Contains(d.Label, "video3") {
				video3DID = d.DeviceID
			}
			if strings.Contains(d.Label, "video4") {
				video4DID = d.DeviceID
			}
		}
	}

	if video3DID == "" || video4DID == "" {
		log.Fatal("video3 or video4 device not found")
	}

	/* -------------------- Initial Stream -------------------- */

	stream, err := getVideoStream(video3DID, codecSelector)
	if err != nil {
		log.Fatal(err)
	}

	client.VideoStream = stream
	track := stream.GetVideoTracks()[0]

	sender, err := pc.AddTrack(track)
	if err != nil {
		log.Fatal(err)
	}
	client.VideoSender = sender

	/* -------------------- Signaling -------------------- */

	offer, _ := pc.CreateOffer(nil)
	_ = pc.SetLocalDescription(offer)

	msg, _ := json.Marshal(MessageOut{Type: "offer", Data: offer})
	client.MessageChan <- msg

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		msg, _ := json.Marshal(MessageOut{Type: "ice", Data: c.ToJSON()})
		client.MessageChan <- msg
	})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		handleWSMessage(client, msg, codecSelector)
	}
}

/* ----------------------------- main ----------------------------- */

func main() {
	http.HandleFunc("/ws", handleWSConnection)
	log.Println("Server started on :9091")
	log.Fatal(http.ListenAndServe(":9091", nil))
}