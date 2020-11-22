package test

import (
	"os"
	"flag"
	"net/url"
	"encoding/json"
	"log"
	"time"
	"io"
	"fmt"
	"strconv"
	"context"

	"github.com/pion/webrtc/v3"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3/pkg/media"
	//"github.com/pion/webrtc/v3/pkg/media/ivfreader"
	"github.com/pion/webrtc/v3/pkg/media/oggreader"
)

const (
	audioFileName = "output.ogg"
)

type RequestBody struct {
    Action string `json:"action"`
    UserID string `json:"user_id"`
    RoomID string `json:"room_id"`
    Body map[string]string `json:"body"`
    SDP webrtc.SessionDescription `json:"sdp"`
    ICE_Candidate *webrtc.ICECandidate `json:"ice_candidate"`

}

var addr = flag.String("addr", "localhost:8080", "http service address")
var peerConnection *webrtc.PeerConnection

func newPeerConnection(i int) {
	// _, err := os.Stat(videoFileName)
	// haveVideoFile := !os.IsNotExist(err)

	_, err := os.Stat(audioFileName)
	haveAudioFile := !os.IsNotExist(err)

	if !haveAudioFile{
		panic("Could not find `" + audioFileName + "` or `" + videoFileName + "`")
	}



	//Create a websocket connection
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	// defer c.Close()

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
        fmt.Println("Track recieved")
    })

	go func() {
		for {
			_, message, _ := c.ReadMessage()
			//fmt.Printf("Type: %T, Message: %v", err, err)
			reqBody := constant.RequestBody{}
		    json.Unmarshal(message, &reqBody)

		    action_type := reqBody.Action
		    log.Println("[TEST] - Message recieved with action : ", action_type)
		    fmt.Println(reqBody.SDP)
		    switch action_type {

		    case "SERVER_ANSWER":
		        peerConnection.SetRemoteDescription(reqBody.SDP)

		    // case "SERVER_OFFER":
		    //     peerConnection.SetRemoteDescription(reqBody.SDP)
		    //     ans, _ := peerConnection.CreateAnswer(nil)
		    //     peerConnection.SetLocalDescription(ans)

		    //     reqBody := constant.RequestBody{}
			   //  reqBody.Action = "CLIENT_ANSWER"
			   //  reqBody.SDP = ans
			   //  reqBody.RoomID = "Room - 1"
			   //  reqBody.UserID = "User-001"
			   //  off, _ := json.Marshal(reqBody)
			   //  log.Println("[TEST] - SDP Answer Sent")
			   //  c.WriteMessage(websocket.TextMessage, off)

		    // case "NEW_ICE_CANDIDATE_CLIENT":
		    //     action.AddIceCandidate(router, reqBody)
		    }
		}
	}()
	

	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	if haveAudioFile {
		// Create a audio track
		audioTrack, audioTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion")
		if audioTrackErr != nil {
			panic(audioTrackErr)
		}
		if _, audioTrackErr = peerConnection.AddTrack(audioTrack); audioTrackErr != nil {
			panic(audioTrackErr)
		}

		go func() {
			// Open a IVF file and start reading using our IVFReader
			file, oggErr := os.Open(audioFileName)
			if oggErr != nil {
				panic(oggErr)
			}

			// Open on oggfile in non-checksum mode.
			ogg, _, oggErr := oggreader.NewWith(file)
			if oggErr != nil {
				panic(oggErr)
			}

			// Wait for connection established
			<-iceConnectedCtx.Done()

			// Keep track of last granule, the difference is the amount of samples in the buffer
			var lastGranule uint64
			for {
				pageData, pageHeader, oggErr := ogg.ParseNextPage()
				if oggErr == io.EOF {
					fmt.Printf("All audio pages parsed and sent")
					os.Exit(0)
				}

				if oggErr != nil {
					panic(oggErr)
				}

				// The amount of samples is the difference between the last and current timestamp
				sampleCount := float64(pageHeader.GranulePosition - lastGranule)
				lastGranule = pageHeader.GranulePosition
				sampleDuration := time.Duration((sampleCount/48000)*1000) * time.Millisecond

				if oggErr = audioTrack.WriteSample(media.Sample{Data: pageData, Duration: sampleDuration}); oggErr != nil {
					panic(oggErr)
				}

				time.Sleep(sampleDuration)
			}
		}()
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	
	gatherCompletePromise := webrtc.GatheringCompletePromise(peerConnection)

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}
	<-gatherCompletePromise


	//Send SDP Offer
    reqBody := RequestBody{}
    reqBody.Action = "INIT"
    reqBody.SDP = offer
    reqBody.RoomID = "Room - 1"
    reqBody.UserID = strconv.Itoa(i)
    off, _ := json.Marshal(reqBody)
    log.Println("[TEST] - SDP Offer Sent")
    c.WriteMessage(websocket.TextMessage, off)

    select{}

}

func main() {

		for i :=0; i < 1; i++{
			newPeerConnection(i)
		}
		
}
