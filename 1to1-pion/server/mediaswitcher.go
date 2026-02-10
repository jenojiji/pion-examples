package main

import (
	"fmt"
	"log"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

type MediaSwitcher struct {
	outTrack     *webrtc.TrackLocalStaticRTP
	packetChan   chan *rtp.Packet
	activeSource int
}

func NewMediaSwitcher(outTrack *webrtc.TrackLocalStaticRTP) *MediaSwitcher {
	ms := &MediaSwitcher{
		outTrack:   outTrack,
		packetChan: make(chan *rtp.Packet,100),
	}
	go ms.writer()
	return ms
}

func (ms *MediaSwitcher) writer() {
	for {
		pkt := <-ms.packetChan
		if err := ms.outTrack.WriteRTP(pkt); err != nil {
			log.Println("write RTP error:", err)
			return
		}
	}
}

func (ms *MediaSwitcher) SwitchTo(sourceID int, pc *webrtc.PeerConnection, tr *webrtc.TrackRemote) {
	fmt.Println("switchto func")
	if ms.activeSource == sourceID {
		return
	}

	ms.activeSource = sourceID

	if tr.Kind() == webrtc.RTPCodecTypeVideo {
		_ = pc.WriteRTCP([]rtcp.Packet{
			&rtcp.PictureLossIndication{
				MediaSSRC: uint32(tr.SSRC()),
			},
		})
	}
}
