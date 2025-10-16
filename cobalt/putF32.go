package cobalt

import (
	"math"
	"encoding/binary"
)

func putF32(buf []byte, off int, v float32) {
	binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(v))
}
