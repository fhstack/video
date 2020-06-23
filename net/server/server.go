package main

import (
	"flag"
	"fmt"
	"github.com/l-f-h/rudp"
	"github.com/l-f-h/video/codec"
	"github.com/veandco/go-sdl2/sdl"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	_ "net/http/pprof"
)

func main() {
	var protocol string
	flag.StringVar(&protocol, "p", "unknown", "udp/rudp/tcp")
	flag.Parse()
	go func() {
		log.Println(http.ListenAndServe("localhost:9999", nil))
	}()
	switch protocol {
	case "tcp":
		sdl.Main(tcp)
	case "udp":
		sdl.Main(udp)
	case "rudp":
		sdl.Main(rUDP)
	default:
		log.Fatalf("protocol error")
	}
}

func tcp() {
	listen, err := net.Listen("tcp", "127.0.0.1:8888")
	if err != nil {
		log.Fatalf("net.Listen tcp error: %v", err)
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Fatalf("listen.Accept error: %v", err)
		}
		go func(c net.Conn) {
			decodeH264Stream(c)
		}(conn)

	}
}

func udp() {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: 8888,
	})
	if err != nil {
		log.Fatalf("net.Listen udp error: %v", err)
	}
	decodeH264Stream(conn)
}

func rUDP() {
	rudp.Debug()
	listener, err := rudp.ListenRUDP(&net.UDPAddr{
		Port: 8888,
	})
	if err != nil {
		log.Fatalf("rudp.ListenRUDP error: %v", err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("listener.Accept error: %v", err)
		}
		go func(c net.Conn) {
			decodeH264Stream(c)
		}(conn)
	}
}

func decodeH264Stream(conn net.Conn) {
	over := make(chan struct{})
	codecHandler := codec.NewCodecHandler()
	if err := codecHandler.InitAndOpenH264Decoder(); err != nil {
		log.Fatalf("InitAndOpenH264Decoder error: %v", err)
	}

	go codecHandler.H264Decode()

	go func() {
		defer func() {
			over <- struct{}{}
		}()

		for {
			data := make([]byte, 1024*30)
			n, err := conn.Read(data)
			if err != nil {
				if err == io.EOF {
					return
				}
				log.Fatalf("ReadFrom error: %v", err)
			}
			data = data[:n]
			// fmt.Println(n)
			codecHandler.PushRawData(data)
		}
	}()

	var (
		window     = &sdl.Window{}
		renderCtx  = &sdl.Renderer{}
		textureCtx = &sdl.Texture{}
	)

	sdl.Do(func() {
		err := sdl.Init(sdl.INIT_AUDIO | sdl.INIT_VIDEO | sdl.INIT_TIMER)
		if err != nil {
			log.Fatalf("sdl.Init error: %v", err)
		}
		window, renderCtx, err = sdl.CreateWindowAndRenderer(
			1280,
			720,
			sdl.WINDOW_SHOWN)
		if err != nil {
			log.Fatalf("sdl.CreateWindow error: %v", err)
		}
		window.SetTitle("Video From LFH")
		textureCtx, err = renderCtx.CreateTexture(sdl.PIXELFORMAT_IYUV, sdl.TEXTUREACCESS_TARGET,
			1280, 720)
		if err != nil {
			log.Fatalf("renderCtx.CreateTexture error: %v", err)
		}
		fmt.Println("sdl init successful")
	})

	go func() {
		yuvLineSize := codecHandler.GetYUVFrameLineSize()
		yuvImageQue := codecHandler.YUVImgRecQue()
		for yuvImg := range yuvImageQue {
			if err := textureCtx.UpdateYUV(nil,
				yuvImg.Y,
				int(yuvLineSize[0]),
				yuvImg.Cb,
				int(yuvLineSize[1]),
				yuvImg.Cr,
				int(yuvLineSize[2]),
			); err != nil {
				fmt.Printf("textureCtx.UpdateYUV error: %v\n", err)
				return
			}
			if err := renderCtx.Copy(textureCtx, nil, nil); err != nil {
				fmt.Printf("renderCtx.Copy error: %v\n", err)
				continue
			}
			renderCtx.Present()
			sdl.Delay(uint32(math.Floor(codec.PerFrameDelayOf30FPS)))
		}
	}()

	sdl.Do(func() {
		defer func() {
			window.Destroy()
			textureCtx.Destroy()
			renderCtx.Destroy()
			sdl.Quit()
		}()
		running := true
		for running {
			for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
				switch event.(type) {
				case *sdl.QuitEvent:
					fmt.Println("Quit")
					running = false
				}
			}
			select {
			case <-over:
				fmt.Println("over Quit")
				return
			}
		}
	})
}
