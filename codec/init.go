package codec

import "github.com/giorgisio/goav/avformat"

func init() {
	avformat.AvRegisterAll()
}