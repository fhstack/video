package codec

import (
	"errors"
	"image"
	"unsafe"

	"github.com/giorgisio/goav/avutil"
)

// support yuv420 pic
func frameToYUVPic(frame *avutil.Frame) (*image.YCbCr, error) {
	w, h, linesize, data := avutil.AvFrameGetInfo(frame)
	if data[0] == nil || data[1] == nil || data[2] == nil {
		return nil, errors.New("frame data error")
	}

	r := image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: w, Y: h}}

	img := image.NewYCbCr(r, image.YCbCrSubsampleRatio420)
	img.Y = make([]byte, w*h)
	for i, p := 0, data[0]; i < w*h; i++ {
		img.Y[i] = *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(i)))
	}

	wCb := int(linesize[1]) / 2
	img.Cb = make([]byte, wCb*h)
	for i, p := 0, data[1]; i < wCb*h; i++ {
		img.Cb[i] = *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(i)))
	}

	wCr := int(linesize[2]) / 2
	img.Cr = make([]byte, wCr*h)
	for i, p := 0, data[2]; i < wCr*h; i++ {
		img.Cr[i] = *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(i)))
	}
	return img, nil
}
