package cobalt

import (
	"fmt"

	"github.com/cogentcore/webgpu/wgpu"
)

// Frame buffer textures automatically resize to match the cobalt viewport.

type FrameBufferNode struct {
	Label    string
	Format   wgpu.TextureFormat
	Usage    wgpu.TextureUsage
	MipCount uint32
	Material *Texture // the view this layer renders into
}

func (t *FrameBufferNode) Init(c *State) error {
	tex, err := CreateTexture(c, t.Label, 1, 1, t.MipCount, t.Format, t.Usage)
	if err != nil {
		return err
	}

	t.Material = tex

	return nil
}

func (t *FrameBufferNode) GetType() string {
	return "cobalt:framebuffer"
}

func (t *FrameBufferNode) IsEnabled() bool {
	return true
}

// view is the backing frame texture view that is created each frame
func (t *FrameBufferNode) OnRun(c *State, encoder *wgpu.CommandEncoder, view *wgpu.TextureView) error {
	return nil
}

func (t *FrameBufferNode) OnDestroy(c *State) error {
	if t.Material != nil {
		t.Material.Texture.Release()
		t.Material = nil
	}

	return nil
}

func (t *FrameBufferNode) OnViewportPosition(c *State) error {
	return nil
}

func (t *FrameBufferNode) OnResize(c *State) error {
	t.OnDestroy(c)

    fmt.Println("Resizing framebuffer node. width:", c.Viewport.width, "height:", c.Viewport.height)

	tex, err := CreateTexture(c, t.Label, c.Viewport.width,  c.Viewport.height, t.MipCount, t.Format, t.Usage)
	if err != nil {
		return err
	}

	t.Material = tex

	return nil
}
