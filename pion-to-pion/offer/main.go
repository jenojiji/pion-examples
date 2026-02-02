package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pion/randutil"
	"github.com/pion/webrtc/v4"
)

func signalCandidate(addr string, candidate *webrtc.ICECandidate) error {
	payload := []byte(candidate.ToJSON().Candidate)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/candidate", addr),
		"application/json; charset=utf-8",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}

	return resp.Body.Close()
}

func main() {
	offerAddr := flag.String("offer-address", ":50000", "Address that the Offer HTTP server is hosted on.")
	answerAddr := flag.String("answer-address", "127.0.0.1:60000", "Address that the Answer HTTP server is hosted on.")
	flag.Parse()

	var candidateMux sync.Mutex
	pendingCandidates := make([]*webrtc.ICECandidate, 0)

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer func() {
		if closeErr := peerConnection.Close(); closeErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", closeErr)
		}
	}()

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateMux.Lock()
		defer candidateMux.Unlock()

		desc := peerConnection.RemoteDescription()
		if desc == nil {
			pendingCandidates = append(pendingCandidates, candidate)
		} else if onICECandidateErr := signalCandidate(*answerAddr, candidate); onICECandidateErr != nil {
			panic(onICECandidateErr)
		}
	})

	http.HandleFunc("/candidate", func(res http.ResponseWriter, req *http.Request) {
		candidate, candidateErr := io.ReadAll(req.Body)
		if candidateErr != nil {
			panic(candidateErr)
		}
		if candidateErr := peerConnection.AddICECandidate(
			webrtc.ICECandidateInit{Candidate: string(candidate)},
		); candidateErr != nil {
			panic(candidateErr)
		}
	})

	http.HandleFunc("/sdp", func(res http.ResponseWriter, req *http.Request) {
		sdp := webrtc.SessionDescription{}
		if decodeErr := json.NewDecoder(req.Body).Decode(&sdp); decodeErr != nil {
			panic(decodeErr)
		}

		if setErr := peerConnection.SetRemoteDescription(sdp); setErr != nil {
			panic(setErr)
		}

		candidateMux.Lock()
		defer candidateMux.Unlock()

		for _, c := range pendingCandidates {
			if onICECandidateErr := signalCandidate(*answerAddr, c); onICECandidateErr != nil {
				panic(onICECandidateErr)
			}
		}
	})

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", state.String())

		if state == webrtc.PeerConnectionStateFailed {
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}

		if state == webrtc.PeerConnectionStateClosed {
			fmt.Println("Peer Connection has gone to closed exiting")
			os.Exit(0)
		}
	})

	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}
	setupDataChannel(dataChannel)

	go func() { panic(http.ListenAndServe(*offerAddr, nil)) }()

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}
	payload, err := json.Marshal(offer)
	if err != nil {
		panic(err)
	}
	resp, err := http.Post(
		fmt.Sprintf("http://%s/sdp", *answerAddr),
		"application/json; charset=utf-8",
		bytes.NewReader(payload),
	)
	if err != nil {
		panic(err)
	} else if err := resp.Body.Close(); err != nil {
		panic(err)
	}

	select {}
}

func setupDataChannel(dataChannel *webrtc.DataChannel) {
	// Register channel opening handling
	dataChannel.OnOpen(func() {
		fmt.Printf(
			"Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n",
			dataChannel.Label(), dataChannel.ID(),
		)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			message, sendTextErr := randutil.GenerateCryptoRandomString(
				15, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			)
			if sendTextErr != nil {
				panic(sendTextErr)
			}

			// Send the message as text
			fmt.Printf("Sending '%s'\n", message)
			if sendTextErr = dataChannel.SendText(message); sendTextErr != nil {
				panic(sendTextErr)
			}
		}
	})

	// Register text message handling
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
	})
}
