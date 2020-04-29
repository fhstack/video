package codec

//Package codec provides codec for video

import (
	"errors"
	"fmt"
	"github.com/giorgisio/goav/avcodec"
	"github.com/giorgisio/goav/avformat"
	"github.com/giorgisio/goav/avutil"
	"github.com/giorgisio/goav/swscale"
	"image"
	"log"
	"unsafe"
)

type codecHandler struct {
	formatContext *avformat.Context
	videoStreamNb int // number of the video stream
	codecCtx      *avcodec.Context
	frameYUV      *avutil.Frame
	swsCtx        *swscale.Context
	frameRAW      *avutil.Frame     // just avoid alloc frame frequently
	yuvImgQueue   chan *image.YCbCr // notify codec
}

func NewCodecHandler() *codecHandler {
	return &codecHandler{
		frameRAW:    avutil.AvFrameAlloc(),
		yuvImgQueue: make(chan *image.YCbCr, 1<<10)}
}

func (h *codecHandler) InitFormatContextWithVideoURI(uri string) error {
	formatContext := avformat.AvformatAllocContext()
	if errno := avformat.AvformatOpenInput(&formatContext, uri, nil, nil); errno != 0 {
		return errors.New("avformat.AvformatOpenInput error: " + avutil.ErrorFromCode(errno).Error())
	}

	if errno := formatContext.AvformatFindStreamInfo(nil); errno != 0 {
		return errors.New("formatContext.AvformatFindStreamInfo: " + avutil.ErrorFromCode(errno).Error())

	}
	formatContext.AvDumpFormat(0, uri, 0)
	h.formatContext = formatContext
	return nil
}

func (h *codecHandler) InitAndOpenVideoCodecCtx() error {
	videoStream := -1
	for i, streams := 0, h.formatContext.Streams(); i < int(h.formatContext.NbStreams()); i++ {
		if streams[i].Codec().GetCodecType() == avformat.AVMEDIA_TYPE_VIDEO {
			videoStream = i
			break
		}
	}
	if videoStream < 0 {
		return errors.New("not found video stream")
	}
	h.videoStreamNb = videoStream

	codecCtxOri := h.formatContext.Streams()[videoStream].Codec()
	codec := avcodec.AvcodecFindDecoder(avcodec.CodecId(codecCtxOri.GetCodecId()))
	if codec == nil {
		return errors.New("avcodec.AvcodecFindDecoder not found decoder for video stream")
	}

	codecCtx := codec.AvcodecAllocContext3()
	if errno := codecCtx.AvcodecCopyContext((*avcodec.Context)(unsafe.Pointer(codecCtxOri))); errno < 0 {
		return errors.New("codecCtx.AvcodecCopyContext error: " + avutil.ErrorFromCode(errno).Error())
	}
	if errno := codecCtx.AvcodecOpen2(codec, nil); errno < 0 {
		return errors.New("codecCtx.AvcodecOpen2 error: " + avutil.ErrorFromCode(errno).Error())
	}

	h.codecCtx = codecCtx
	return nil
}

func (h *codecHandler) InitYUVFrameContainer() error {
	frameYUV := avutil.AvFrameAlloc()
	if frameYUV == nil {
		return errors.New("avutil.AvFrameAlloc failed")
	}

	numBytes := uintptr(avcodec.AvpictureGetSize(avcodec.AV_PIX_FMT_YUV, h.codecCtx.Width(), h.codecCtx.Height()))
	buffer := avutil.AvMalloc(numBytes)
	avpicture := (*avcodec.Picture)(unsafe.Pointer(frameYUV))
	if errno := avpicture.AvpictureFill((*uint8)(buffer), avcodec.AV_PIX_FMT_YUV,
		h.codecCtx.Width(), h.codecCtx.Height()); errno < 0 {
		return fmt.Errorf("avpicture.AvpictureFill error: %v", avutil.ErrorFromCode(errno))
	}

	if err := avutil.AvSetFrame(frameYUV, h.codecCtx.Width(), h.codecCtx.Height(), avcodec.AV_PIX_FMT_YUV); err != nil {
		return fmt.Errorf("avutil.AvSetFrame error: %v", err)
	}
	h.frameYUV = frameYUV
	return nil
}

func (h *codecHandler) InitSwsContext() {
	// software scaling Context	init
	h.swsCtx = swscale.SwsGetcontext(
		h.codecCtx.Width(),
		h.codecCtx.Height(),
		swscale.PixelFormat(h.codecCtx.PixFmt()),
		h.codecCtx.Width(),
		h.codecCtx.Height(),
		avcodec.AV_PIX_FMT_YUV,
		avcodec.SWS_BILINEAR,
		nil,
		nil,
		nil,
	)
}

// Run resolve frame from video and push packet to codec
func (h *codecHandler) Run() {
	go func() {
		defer close(h.yuvImgQueue)
		packet := avcodec.AvPacketAlloc()
		yuvLineSize := avutil.Linesize(h.frameYUV)
		for h.formatContext.AvReadFrame(packet) >= 0 {
			if packet.StreamIndex() != h.videoStreamNb {
				continue
			}
			if errno := h.codecCtx.AvcodecSendPacket(packet); errno < 0 {
				log.Printf("AvcodecSendPacket error: %v\n", avutil.ErrorFromCode(errno))
				return
			}

			for {
				if errno := h.codecCtx.AvcodecReceiveFrame((*avcodec.Frame)(unsafe.Pointer(h.frameRAW))); errno == avutil.AvErrorEAGAIN || errno == avutil.AvErrorEOF {
					break
				} else if errno < 0 {
					log.Printf("AvcodecReceiveFrame error: %v\n", avutil.ErrorFromCode(errno))
					return
				}
				rawLineSize := avutil.Linesize(h.frameRAW)

				if errno := swscale.SwsScale2(h.swsCtx, avutil.Data(h.frameRAW),
					rawLineSize, 0, h.codecCtx.Height(),
					avutil.Data(h.frameYUV), yuvLineSize); errno < 0 {
					log.Printf("SwsScale2 error: %v\n", avutil.ErrorFromCode(errno))
					return
				}

				yuvImg, err := avutil.GetPicture(h.frameYUV)
				if err != nil {
					log.Printf("avutil.GetPicture error: %v\n",err)
					return
				}
				h.yuvImgQueue <- yuvImg
			}
		}
	}()
}

func (h *codecHandler) YUVImgRecQue() <-chan *image.YCbCr{
	return h.yuvImgQueue
}

func (h *codecHandler) GetVideoWidth() int32 {
	return int32(h.codecCtx.Width())
}

func (h *codecHandler) GetVideoHeight() int32 {
	return int32(h.codecCtx.Height())
}

func (h *codecHandler) GetYUVFrameLineSize() [8]int32 {
	return avutil.Linesize(h.frameYUV)
}

// TODO
// Free free the codec resource
func (h *codecHandler) Free() {

}
