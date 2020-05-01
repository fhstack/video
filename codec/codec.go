package codec

//Package codec provides codec only for video, not support audio now

import (
	"errors"
	"fmt"
	"image"
	"log"
	"unsafe"

	"github.com/giorgisio/goav/avcodec"
	"github.com/giorgisio/goav/avformat"
	"github.com/giorgisio/goav/avutil"
	"github.com/giorgisio/goav/swscale"
)

type codecHandler struct {
	formatContext *avformat.Context
	videoStreamNb int              // number of the video stream
	codecCtx      *avcodec.Context // ctx of decoder or encoder
	frameYUV      *avutil.Frame
	swsCtx        *swscale.Context
	yuvImgQueue   chan *image.YCbCr
}

func NewCodecHandler() *codecHandler {
	return &codecHandler{
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

func (h *codecHandler) FindVideoStream() error {
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
	return nil
}

func (h *codecHandler) InitAndOpenVideoDecoder() error {
	codecCtxOri := h.formatContext.Streams()[h.videoStreamNb].Codec()
	decoder := avcodec.AvcodecFindDecoder(avcodec.CodecId(codecCtxOri.GetCodecId()))
	if decoder == nil {
		return errors.New("avcodec.AvcodecFindDecoder not found decoder for video stream")
	}

	decoderCtx := decoder.AvcodecAllocContext3()
	if errno := decoderCtx.AvcodecCopyContext((*avcodec.Context)(unsafe.Pointer(codecCtxOri))); errno < 0 {
		return fmt.Errorf("codecCtx.AvcodecCopyContext error: %v", avutil.ErrorFromCode(errno))
	}
	if errno := decoderCtx.AvcodecOpen2(decoder, nil); errno < 0 {
		return fmt.Errorf("codecCtx.AvcodecOpen2 error: %v", avutil.ErrorFromCode(errno))
	}
	h.codecCtx = decoderCtx
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

// Run read frame from video, push the frame packet to codec, and append YUVPic to queue
func (h *codecHandler) Run() {
	go func() {
		defer close(h.yuvImgQueue)
		packet := avcodec.AvPacketAlloc()
		yuvLineSize := avutil.Linesize(h.frameYUV)
		frameRAW := avutil.AvFrameAlloc()
		for h.formatContext.AvReadFrame(packet) >= 0 {
			if packet.StreamIndex() != h.videoStreamNb {
				continue
			}
			if errno := h.codecCtx.AvcodecSendPacket(packet); errno < 0 {
				log.Printf("AvcodecSendPacket error: %v\n", avutil.ErrorFromCode(errno))
				return
			}
			for {
				if errno := h.codecCtx.AvcodecReceiveFrame((*avcodec.Frame)(unsafe.Pointer(frameRAW))); errno == avutil.AvErrorEAGAIN || errno == avutil.AvErrorEOF {
					break
				} else if errno < 0 {
					log.Printf("AvcodecReceiveFrame error: %v\n", avutil.ErrorFromCode(errno))
					return
				}

				rawLineSize := avutil.Linesize(frameRAW)
				if errno := swscale.SwsScale2(h.swsCtx, avutil.Data(frameRAW),
					rawLineSize, 0, h.codecCtx.Height(),
					avutil.Data(h.frameYUV), yuvLineSize); errno < 0 {
					log.Printf("SwsScale2 error: %v\n", avutil.ErrorFromCode(errno))
					return
				}

				yuvImg, err := avutil.GetPicture(h.frameYUV)
				if err != nil {
					log.Printf("avutil.GetPicture error: %v\n", err)
					return
				}
				h.yuvImgQueue <- yuvImg
			}
		}
	}()
}

// GetPerFrameDuration calculate the duration of one frame, ms
func (h *codecHandler) GetPerFrameDuration() uint32 {
	timeBase := float64(h.codecCtx.AvCodecGetPktTimebase2().Num()) / float64(h.codecCtx.AvCodecGetPktTimebase2().Den())
	return uint32(timeBase * 1000000)
}

func (h *codecHandler) YUVImgRecQue() <-chan *image.YCbCr {
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