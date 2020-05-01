package codec

import (
	"github.com/giorgisio/goav/avdevice"
	"github.com/giorgisio/goav/avformat"
)

func init() {
	avformat.AvRegisterAll()
	avdevice.AvdeviceRegisterAll()
}