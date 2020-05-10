package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"unsafe"

	"github.com/l-f-h/video/cam"
	"github.com/l-f-h/video/codec"
	"github.com/veandco/go-sdl2/sdl"
	_ "net/http/pprof"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:10000", nil))
	}()
	sdl.Main(udp)
}

func tcp() {
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{
		Port: 8888,
	})

	if err != nil {
		log.Fatalf("net.DialTCP error: %v", err)
	}

	transmit(conn)
}

func udp() {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		Port: 8888,
	})

	if err != nil {
		log.Fatalf("net.DialUDP error: %v", err)
	}

	if err := conn.SetWriteBuffer(65536); err != nil {
		log.Fatalf("conn.SetWriteBuffer error: %v", err)
	}

	transmit(conn)
}

func transmit(conn net.Conn) {
	codecHandler := codec.NewCodecHandler()
	if err := codecHandler.InitH264Encoder(); err != nil {
		log.Fatalf("InitH264Encoder err: %v", err)
	}

	webcam, err := cam.NewWebCamWithLocalCam()
	if err != nil {
		log.Fatalf("NewWebCamWithLocalCam error: %v", err)
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, os.Kill)
	go func() {
		<-ch
		webcam.Stop()
		codecHandler.Stop()
		os.Exit(-1)
	}()

	sdl.Do(webcam.Start)
	go func() {
		for frame := range webcam.FrameQueue() {
			if err := codecHandler.H264EncoderInputRGBImage(frame); err != nil {
				log.Fatalf("H264EncoderInputRGBImage error: %v", err)
			}
		}
	}()

	// transmit the h264 frame
	go func() {
		for p := range codecHandler.GetH264EncoderOutputPacketQueue() {
			shd := reflect.SliceHeader{}
			shd.Data = uintptr(unsafe.Pointer(p.Data()))
			shd.Len = p.Size()
			shd.Cap = p.Size()
			data := *(*[]byte)(unsafe.Pointer(&shd))
			//encodedStr := hex.EncodeToString(data)
			//fmt.Println(encodedStr)
			//fmt.Println(len(data))
			_, err := conn.Write(data)
			if err != nil {
				log.Fatalf("write error: %v", err)
			}
		}
	}()

	<-ch
}
