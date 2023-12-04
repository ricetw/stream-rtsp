package main

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/deepch/vdk/format/rtspv2"
)

var (
	ErrorStreamExitNoVideoOnStream = errors.New("no video")
	ErrorStreamExitRtspDisconnect  = errors.New("rtsp disconnected")
	ErrorStreamExitNoViewer        = errors.New("vod no viewer")
)

func serveStreams() {
	for k, v := range Config.Streams {
		if !v.OnDemand {
			go RTSPWorkerLoop(k, os.Getenv("RTSP_URL"), v.OnDemand, v.DisableAudio, v.Debug)
		}
	}
}
func RTSPWorkerLoop(name, url string, OnDemand, DisableAudio, Debug bool) {
	defer Config.RunUnlock(name)
	for {
		log.Println("connect:", name)
		err := RTSPWorker(name, url, OnDemand, DisableAudio, Debug)
		if err != nil {
			log.Println("error:", err)
		}
		if OnDemand && !Config.HasViewer(name) {
			log.Println("error:", "vod no viewer")
			return
		}
		time.Sleep(Config.Server.RetryConnect * time.Second)
	}
}
func RTSPWorker(name, url string, OnDemand, DisableAudio, Debug bool) error {
	keyTest := time.NewTimer(20 * time.Second)
	clientTest := time.NewTimer(20 * time.Second)
	//add next TimeOut
	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: url, DisableAudio: DisableAudio, DialTimeout: 3 * time.Second, ReadWriteTimeout: 3 * time.Second, Debug: Debug})
	if err != nil {
		return err
	}
	defer RTSPClient.Close()
	if RTSPClient.CodecData != nil {
		Config.coAd(name, RTSPClient.CodecData)
	}
	var AudioOnly bool
	if len(RTSPClient.CodecData) == 1 && RTSPClient.CodecData[0].Type().IsAudio() {
		AudioOnly = true
	}
	for {
		select {
		case <-clientTest.C:
			if OnDemand && !Config.HasViewer(name) {
				return ErrorStreamExitNoViewer
			}
		case <-keyTest.C:
			return ErrorStreamExitNoVideoOnStream
		case signals := <-RTSPClient.Signals:
			switch signals {
			case rtspv2.SignalCodecUpdate:
				Config.coAd(name, RTSPClient.CodecData)
			case rtspv2.SignalStreamRTPStop:
				return ErrorStreamExitRtspDisconnect
			}
		case packetAV := <-RTSPClient.OutgoingPacketQueue:
			if AudioOnly || packetAV.IsKeyFrame {
				keyTest.Reset(20 * time.Second)
			}
			Config.cast(name, *packetAV)
		}
	}
}
