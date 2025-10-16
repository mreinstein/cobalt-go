package cobalt

import "github.com/cogentcore/webgpu/wgpu"
import "fmt"

type TextureDimensions struct {
	Width  int
	Height int
}

type Texture struct {
	Size    TextureDimensions
	Texture *wgpu.Texture
	View    *wgpu.TextureView
	MipView []wgpu.TextureView
	Sampler *wgpu.Sampler
}

type TextureMipView struct {
	label           string
	format          string
	dimension       string
	aspect          string
	baseMipLevel    int
	mipLevelCount   int
	baseArrayLayer  int
	arrayLayerCount int
}

// TODO: maybe it would be better to rename this material since it has more than just a texture in it

func CreateTexture(c *State, label string, width int, height int, mip_count uint32, format wgpu.TextureFormat, usage wgpu.TextureUsage) (*Texture, error) {
	t := &Texture{}

fmt.Println("mip_count:", mip_count, "tex format:", format)

	t.Size.Width = width
	t.Size.Height = height

	texDesc := wgpu.TextureDescriptor{
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		Format:        format,
		Usage:         usage,
		MipLevelCount: mip_count,
		Dimension:     wgpu.TextureDimension2D,
		SampleCount:   1,
	}
	texture, err := c.Device.CreateTexture(&texDesc)
	if err != nil {
		return nil, err
	}

	t.Texture = texture

	view, err := t.Texture.CreateView(nil)
	if err != nil {
		return nil, err
	}

	t.View = view
	for i := uint32(0); i < mip_count; i++ {
		mipV, err := texture.CreateView(&wgpu.TextureViewDescriptor{
			Label:           label,
			Format:          format,                      // inherit from texture if Undefined
			Dimension:       wgpu.TextureViewDimension2D, // 1D, 2D, 2DArray, Cube, etc.
			Aspect:          wgpu.TextureAspectAll,       // All, StencilOnly, DepthOnly
			BaseMipLevel:    i,
			MipLevelCount:   1, // 0 means "all levels from base"
			BaseArrayLayer:  0,
			ArrayLayerCount: 1, // 0 means "all layers from base"
		})
		if err != nil {
			return nil, err
		}
		t.MipView = append(t.MipView, *mipV)
	}

	sampler, err := c.Device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        label + " sampler",
		AddressModeU: wgpu.AddressModeClampToEdge,
		AddressModeV: wgpu.AddressModeClampToEdge,
		AddressModeW: wgpu.AddressModeClampToEdge,
		MagFilter:    wgpu.FilterModeNearest,
		MinFilter:    wgpu.FilterModeNearest,

		MaxAnisotropy: 1,
		MipmapFilter:  wgpu.MipmapFilterModeNearest,
		// If you created mipmaps, set appropriate lod range:
		// MinLOD: 0, MaxLOD: float32(mipCount-1),
		LodMinClamp:   0,
    	LodMaxClamp:   0, // keeps it at base level

    	Compare:       wgpu.CompareFunctionUndefined,
	})
	if err != nil {
		return nil, err
	}
	t.Sampler = sampler
	return t, nil
}
