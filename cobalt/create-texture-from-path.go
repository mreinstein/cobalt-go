package cobalt

import (
	"errors"
	"image"
	"image/draw"
	"image/png"
	"os"

	"github.com/cogentcore/webgpu/wgpu"
)

func CreateTextureFromPath(c *State, label string, path string, format wgpu.TextureFormat) (*Texture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}

	// Convert to *image.RGBA (straight alpha)
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

	w := rgba.Bounds().Dx()
	h := rgba.Bounds().Dy()
	if w == 0 || h == 0 {
		return nil, errors.New("empty image")
	}

	usage := wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst | wgpu.TextureUsageRenderAttachment
	mipCount := uint32(1)

	t, err := CreateTexture(c, label, w, h, mipCount, format, usage)
	if err != nil {
		return nil, err
	}

	if w == 0 || h == 0 {
		return nil, errors.New("empty image")
	}

	// WebGPU requires bytesPerRow to be a multiple of 256.
	const bpp = 4 // RGBA8
	rowStride := w * bpp
	paddedStride := ((rowStride + 255) / 256) * 256

	var upload []byte
	if rowStride == paddedStride {
		// Already aligned; can upload directly.
		upload = rgba.Pix
	} else {
		// Pad each row out to paddedStride.
		upload = make([]byte, paddedStride*h)
		src := rgba.Pix
		for y := 0; y < h; y++ {
			copy(upload[y*paddedStride:y*paddedStride+rowStride], src[y*rgba.Stride:y*rgba.Stride+rowStride])
		}
	}

	// Upload pixels
	c.Queue.WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  t.Texture,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   wgpu.TextureAspectAll,
		},
		upload,
		&wgpu.TextureDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(paddedStride),
			RowsPerImage: uint32(h),
		},
		&wgpu.Extent3D{
			Width:              uint32(w),
			Height:             uint32(h),
			DepthOrArrayLayers: 1,
		},
	)

	return t, nil
}
