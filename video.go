package main

import (
	"fmt"
	"github.com/l-f-h/video/codec"
	"github.com/veandco/go-sdl2/sdl"
	"log"
)

func main() {
	sdl.Main(Main)
}

func Main() {
	fileName := "./demo.mp4"

	codecHandler := codec.NewCodecHandler()
	if err := codecHandler.InitFormatContextWithVideoURI(fileName); err != nil {
		log.Fatalf("codecHandler.InitFormatContextWithVideoURI error: %v", err)
	}

	if err := codecHandler.InitAndOpenVideoCodecCtx(); err != nil {
		log.Fatalf("codecHandler.InitAndOpenVideoCodecCtx error: %v", err)
	}

	if err := codecHandler.InitYUVFrameContainer(); err != nil {
		log.Fatalf("codecHandler.InitYUVFrameContainer error: %v", err)
	}

	codecHandler.InitSwsContext()

	// Begin
	codecHandler.Run()

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
			sdl.Delay(10)
		}
	}()

	<-ch
	sdl.Quit()
}
