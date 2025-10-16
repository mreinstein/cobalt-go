package cobalt

import (
	"fmt"

	"github.com/cogentcore/webgpu/wgpu"
)

/*
Tile layers are static, and there are usually many of them in several layers.

These use a `TileRenderPass` data structure which provides 100% GPU hardware based tile rendering, making them _almost_ free CPU-wise.

Internally, `TileRenderPass` objects store 1 or more layers, which hold a reference to a sprite texture, and a layer texture.
When a tile layer is drawn, it loads the 2 textures into the gpu.
One of these textures is a lookup table, where each pixel corresponds to a type of sprite.
Because this processing can happen completely in the fragment shader, there's no need to do expensive loops over slow arrays in js land,
which is the typical approach for current state-of-the-art web renderers.

Inspired by/ported from https://blog.tojicode.com/2012/07/sprite-tile-maps-on-gpu.html
*/

type TileLayerNode struct {
	BindGroup     *wgpu.BindGroup
	Material      *Texture
	UniformBuffer *wgpu.Buffer
	Format        wgpu.TextureFormat
	ScrollScale   float32
	TexturePath   string
	LoadOp        string
	TileAtlas     *TileAtlasNode
	// OutputView    *wgpu.TextureView // the view this tile layer renders into. used to be an HDR intermediate texture
}

func (t *TileLayerNode) Init(c *State) error {
	dat := [2]float32{t.ScrollScale, t.ScrollScale}

	uniformBuffer, err := c.Device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "TileLayerBuffer",
		Contents: wgpu.ToBytes(dat[:]),
		Usage:    wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return err
	}

	t.UniformBuffer = uniformBuffer

	e := t.SetTexture(c, t.TexturePath)
	if e != nil {
		return e
	}

	return nil
}

func (t *TileLayerNode) SetTexture(c *State, path string) error {
	if t.Material != nil {
		t.Material.Texture.Release()
		t.Material = nil
	}

	t.TexturePath = path
	fmt.Println("reading tile layer texture from path:", t.TexturePath)

	fmt.Println("fmt:", t.Format)
	material, err := CreateTextureFromPath(c, "tile layer", t.TexturePath, t.Format)
	if err != nil {
		return err
	}

	t.Material = material

	bindGroup, err := c.Device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: &t.TileAtlas.TileBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  t.UniformBuffer,
				Offset:  0,
				Size:    wgpu.WholeSize, // whole buffer
			},
			{
				Binding:     1,
				TextureView: t.Material.View,
			},
			{
				Binding: 2,
				Sampler: t.Material.Sampler,
			},
		},
	})
	if err != nil {
		return err
	}
	t.BindGroup = bindGroup

	return nil
}

func (t *TileLayerNode) GetType() string {
	return "cobalt:tile"
}

func (t *TileLayerNode) IsEnabled() bool {
	return true
}

// view is the backing frame texture view that is created each frame
func (t *TileLayerNode) OnRun(c *State, encoder *wgpu.CommandEncoder, view *wgpu.TextureView) error {
	// calling setTexture can cause the texture to be destroyed followed by this draw command
	// so check for the undefined texture first and bail for a frame.
	if t.Material == nil {
		return nil
	}

	// on the first render, we should clear the color attachment.
	// otherwise load it, so multiple sprite passes can build up data in the color and emissive textures
	renderPass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "tile",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:    view, // t.OutputView,
				LoadOp:  wgpu.LoadOpLoad,
				StoreOp: wgpu.StoreOpStore,
			},
		},
	})
	renderPass.SetPipeline(t.TileAtlas.Pipeline)
	renderPass.SetBindGroup(0, t.BindGroup, nil)
	renderPass.SetBindGroup(1, t.TileAtlas.AtlasBindGroup, nil)
	renderPass.Draw(3, 1, 0, 0) // fullscreen triangle
	_ = renderPass.End()
	renderPass.Release() // must release
	return nil
}

func (t *TileLayerNode) OnDestroy(c *State) error {
	if t.Material == nil {
		return nil
	}
	t.Material.Texture.Release()
	t.Material = nil
	return nil
}

func (t *TileLayerNode) OnViewportPosition(c *State) error {
	return nil
}

func (t *TileLayerNode) OnResize(c *State) error {
	return nil
}
