package main

import (
	"fmt"
	"github.com/l-f-h/video/cam"
	"log"
	"os"
	"os/signal"
	"reflect"
	"unsafe"

	"github.com/l-f-h/video/codec"
	"github.com/veandco/go-sdl2/sdl"
)

func main() {
	sdl.Main(videoEncode)
}

// open cam and encoding the video to h264
func videoEncode() {
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
		<- ch
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

	// output h264 file for testing
	go func() {
		f, err := os.Create("demo.h264")
		if err != nil {
			log.Fatalf("Create output file error: %v", err)
		}
		defer func() {
			f.Sync()
			f.Close()
		}()
		for p := range codecHandler.GetH264EncoderOutputPacketQueue() {
			shd := reflect.SliceHeader{}
			shd.Data = uintptr(unsafe.Pointer(p.Data()))
			shd.Len = p.Size()
			shd.Cap = p.Size()
			data := *(*[]byte)(unsafe.Pointer(&shd))
			_, err := f.Write(data)
			//encodedStr := hex.EncodeToString(data)
			//fmt.Println(encodedStr)
			if err != nil {
				log.Fatalf("write file error: %v", err)
			}
		}
	}()

	<- ch
}

// decode the video and play
func videoDecode() {
	fileName := "./demo.mp4"
	codecHandler := codec.NewCodecHandler()
	if err := codecHandler.InitFormatContextWithVideoURI(fileName); err != nil {
		log.Fatalf("codecHandler.InitFormatContextWithVideoURI error: %v", err)
	}

	if err := codecHandler.FindVideoStream(); err != nil {
		log.Fatalf("codecHandler.FindVideoStream error: %v", err)
	}

	if err := codecHandler.InitAndOpenVideoDecoder(); err != nil {
		log.Fatalf("codecHandler.InitAndOpenVideoCodecCtx error: %v", err)
	}

	// async
	codecHandler.DecoderRun()

	var (
		window     = &sdl.Window{}
		renderCtx  = &sdl.Renderer{}
		textureCtx = &sdl.Texture{}
		err        error
	)

	sdl.Do(func() {
		if err := sdl.Init(sdl.INIT_AUDIO | sdl.INIT_VIDEO | sdl.INIT_TIMER); err != nil {
			log.Fatalf("sdl.Init error: %v", err)
		}
		window, renderCtx, err = sdl.CreateWindowAndRenderer(
			codecHandler.GetVideoWidth(),
			codecHandler.GetVideoHeight(),
			sdl.WINDOW_SHOWN)
		if err != nil {
			log.Fatalf("sdl.CreateWindow error: %v", err)
		}
		window.SetTitle("Video From LFH")
		textureCtx, err = renderCtx.CreateTexture(sdl.PIXELFORMAT_IYUV, sdl.TEXTUREACCESS_TARGET,
			codecHandler.GetVideoWidth(), codecHandler.GetVideoHeight())
		fmt.Println("sdl init successful")
	})

	ch := make(chan struct{})
	go sdl.Do(func() {
		running := true
		for running {
			for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
				switch event.(type) {
				case *sdl.QuitEvent:
					fmt.Println("Quit")
					running = false
					ch <- struct{}{}
				}
			}
		}
	})

	// read frame
	go func() {
		yuvLineSize := codecHandler.GetYUVFrameLineSize()
		yuvImageQue := codecHandler.YUVImgRecQue()
		delay := codecHandler.GetPerFrameDuration()
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
				continue
			}
			if err := renderCtx.Copy(textureCtx, nil, nil); err != nil {
				fmt.Printf("renderCtx.Copy error: %v\n", err)
				continue
			}
			renderCtx.Present()
			sdl.Delay(delay)
		}
	}()

	<-ch
	sdl.Quit()
}
