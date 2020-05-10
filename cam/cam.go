package cam

import (
	"fmt"
	"gocv.io/x/gocv"
	"image"
	"log"
)

const (
	queBuffer = 1 << 5   // avoid to use large buffer
)


type WebCam struct {
	cam      *gocv.VideoCapture
	frameQue chan image.Image
	stop     bool
}

func NewWebCamWithURL(url string) (*WebCam, error) {
	c := &WebCam{}
	c.stop = false
	c.frameQue = make(chan image.Image, queBuffer)
	cam, err := gocv.OpenVideoCapture(url)
	if err != nil {
		return nil, fmt.Errorf("OpenVideoCapture error: %v", err)
	}
	c.cam = cam
	return c, nil
}

func NewWebCamWithLocalCam() (*WebCam, error) {
	c := &WebCam{}
	c.stop = false
	c.frameQue = make(chan image.Image, queBuffer)
	cam, err := gocv.OpenVideoCapture(0)
	if err != nil {
		return nil, fmt.Errorf("OpenVideoCapture error: %v", err)
	}
	c.cam = cam
	return c, nil
}

func (c *WebCam) FrameQueue() <-chan image.Image {
	return c.frameQue
}

func (c *WebCam) Start() {
	go func() {
		defer close(c.frameQue)
		cam, err := gocv.OpenVideoCapture(0)
		if err != nil {
			log.Fatalf("open cam error: %v", err)
		}
		cam.Set(gocv.VideoCaptureFrameWidth, 1280)
		cam.Set(gocv.VideoCaptureFrameHeight, 720)
		img := gocv.NewMat()
		// win := gocv.NewWindow("feihaoCam")
		for {
			if c.stop {
				log.Println("cam stop")
				return
			}
			cam.Read(&img)
			if img.Empty() {
				continue
			}
			if rgbImg, err := img.ToImage(); err != nil {
				log.Printf("convert frame to rgbPic error: %v", err)
				continue
			} else {
				c.frameQue <- rgbImg
			}
		}
	}()
}

func (c *WebCam) Stop() {
	c.stop = true
}
