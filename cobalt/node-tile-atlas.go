package cobalt

// shared tile atlas resource, shared by each tile layer
import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/cogentcore/webgpu/wgpu"
)

//go:embed node-tile.wgsl
var tileWGSL string

type TileAtlasNode struct {
	Pipeline            *wgpu.RenderPipeline
	UniformBuffer       *wgpu.Buffer
	AtlasBindGroup      *wgpu.BindGroup // tile atlas texture, transform UBO
	TileBindGroupLayout wgpu.BindGroupLayout
	TileSize            int
	TileScale           float64
	TexturePath         string
	Format              wgpu.TextureFormat
	AtlasMaterial       *Texture
}

func (t *TileAtlasNode) Init(c *State) error {
	atlasMaterial, err := CreateTextureFromPath(c, "tile atlas", t.TexturePath, t.Format)
	if err != nil {
		return err
	}

	t.AtlasMaterial = atlasMaterial

	buf := [32]byte{} // 332 + 16 *32 in bytes. 32 for common data + (32 max tilelayers * 16 bytes per layer)

	uniformBuffer, err := c.Device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "TileAtlasBuffer",
		Contents: wgpu.ToBytes(buf[:]),
		Usage:    wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return err
	}

	t.UniformBuffer = uniformBuffer

	// --- atlasBindGroupLayout (buffer + texture + sampler) ---
	atlasBGL, err := c.Device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageVertex | wgpu.ShaderStageFragment,
				Buffer: wgpu.BufferBindingLayout{
					Type:             wgpu.BufferBindingTypeUniform, // JS left `{}`; uniform is typical for "uniformBuffer"
					HasDynamicOffset: false,
					MinBindingSize:   0,
				},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageFragment,
				Texture: wgpu.TextureBindingLayout{
					SampleType:    wgpu.TextureSampleTypeFloat, // filterable color
					ViewDimension: wgpu.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
			{
				Binding:    2,
				Visibility: wgpu.ShaderStageFragment,
				Sampler: wgpu.SamplerBindingLayout{
					Type: wgpu.SamplerBindingTypeFiltering,
				},
			},
		},
	})
	if err != nil {
		return err
	}

	atlasBG, err := c.Device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: atlasBGL,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  uniformBuffer,
				Offset:  0,
				Size:    wgpu.WholeSize, // whole buffer
			},
			{
				Binding:     1,
				TextureView: atlasMaterial.View,
			},
			{
				Binding: 2,
				Sampler: atlasMaterial.Sampler,
			},
		},
	})
	if err != nil {
		return err
	}

	t.AtlasBindGroup = atlasBG

	tileBGL, err := c.Device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageVertex | wgpu.ShaderStageFragment,
				Buffer: wgpu.BufferBindingLayout{
					Type:             wgpu.BufferBindingTypeUniform,
					HasDynamicOffset: false,
					MinBindingSize:   0,
				},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageVertex | wgpu.ShaderStageFragment,
				Texture: wgpu.TextureBindingLayout{
					SampleType:    wgpu.TextureSampleTypeFloat,
					ViewDimension: wgpu.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
			{
				Binding:    2,
				Visibility: wgpu.ShaderStageFragment,
				Sampler: wgpu.SamplerBindingLayout{
					Type: wgpu.SamplerBindingTypeFiltering,
				},
			},
		},
	})
	if err != nil {
		return err
	}

	t.TileBindGroupLayout = *tileBGL

	// --- pipeline layout (order matters: [tile, atlas]) ---
	pipelineLayout, err := c.Device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{tileBGL, atlasBGL},
	})
	if err != nil {
		return err
	}

	drawShader, err := c.Device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "tile.wgsl",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{
			Code: tileWGSL,
		},
	})
	if err != nil {
		fmt.Println("shader compilation failed:", err)
		return err
	}
	defer drawShader.Release()

	blend := &wgpu.BlendState{
		Color: wgpu.BlendComponent{
			SrcFactor: wgpu.BlendFactorSrcAlpha,
			DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
			Operation: wgpu.BlendOperationAdd,
		},
		Alpha: wgpu.BlendComponent{
			SrcFactor: wgpu.BlendFactorZero,
			DstFactor: wgpu.BlendFactorOne,
			Operation: wgpu.BlendOperationAdd,
		},
	}

   fmt.Println("heres the color target:", c.Config.Format)
	colorTarget := wgpu.ColorTargetState{
		Format:    c.Config.Format,
		Blend:     blend,
		WriteMask: wgpu.ColorWriteMaskAll,
	}

	// --- pipeline ---
	pipeline, err := c.Device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "tileatlas",
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     drawShader,
			EntryPoint: "vs_main",
			Buffers:    nil, // []wgpu.VertexBufferLayout{} if you have vertex buffers
		},
		Fragment: &wgpu.FragmentState{
			Module:     drawShader,
			EntryPoint: "fs_main",
			Targets:    []wgpu.ColorTargetState{colorTarget},
		},
		Primitive: wgpu.PrimitiveState{
			Topology: wgpu.PrimitiveTopologyTriangleList,
		},
		// DepthStencil, Multisample as needed...
		Multisample: wgpu.MultisampleState{
			Count:                  1,          // <-- fix: 1 for no MSAA (or 4/8 if using MSAA)
			Mask:                   0xFFFFFFFF, // sample mask (all samples enabled)
			AlphaToCoverageEnabled: false,
		},
	})
	if err != nil {
		return err
	}

	t.Pipeline = pipeline

	return nil
}

func (t *TileAtlasNode) GetType() string {
	return "cobalt:tileAtlas"
}

func (t *TileAtlasNode) IsEnabled() bool {
	return true
}

func (t *TileAtlasNode) OnRun(c *State, encoder *wgpu.CommandEncoder, tv *wgpu.TextureView) error {
	return nil
}

func (t *TileAtlasNode) OnDestroy(c *State) error {
	if t.AtlasMaterial == nil {
		return nil
	}
	t.AtlasMaterial.Texture.Release()
	t.AtlasMaterial = nil
	return nil
}

func (t *TileAtlasNode) OnViewportPosition(c *State) error {
	return _writeTileBuffer(c, t)
}

func (t *TileAtlasNode) OnResize(c *State) error {
	return _writeTileBuffer(c, t)
}

func _writeTileBuffer(c *State, t *TileAtlasNode) error {
	// c.Viewport.Position is the top left visible corner of the level
	GAME_WIDTH := float64(c.Viewport.width) / float64(c.Viewport.Zoom)
	GAME_HEIGHT := float64(c.Viewport.height) / float64(c.Viewport.Zoom)
	viewportWidth := float32(GAME_WIDTH / t.TileScale)
	viewportHeight := float32(GAME_HEIGHT / t.TileScale)
	inverseTileSize := 1.0 / t.TileSize
	_buf := [8]float32{
		float32(c.Viewport.position[0]),
		float32(c.Viewport.position[1]),
		viewportWidth,
		viewportHeight,
		1.0 / float32(t.AtlasMaterial.Size.Width),
		1.0 / float32(t.AtlasMaterial.Size.Height),
		float32(t.TileSize),
		float32(inverseTileSize),
	}

	b := make([]byte, 32)
	for i, f := range _buf {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}

	err := c.Queue.WriteBuffer(t.UniformBuffer, 0, b)
	if err != nil {
		return err
	}

	return nil
}
