package cobalt

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/go-gl/mathgl/mgl32"
)

//go:embed node-sprite.wgsl
var spriteWGSL string

// Packed instance layout: 48 bytes (aligned for vec4 fetch)
const INSTANCE_STRIDE = 64

// Offsets inside one instance (bytes)
const (
	OFF_POS      = 0  // float32x2 (8B)
	OFF_SIZE     = 8  // float32x2 (8B)
	OFF_SCALE    = 16 // float32x2 (8B)
	OFF_TINT     = 24 // float32x4 (16B)
	OFF_SPRITEID = 40 // uint32 (4B)
	OFF_OPACITY  = 44 // float32 (4B)
	OFF_ROT      = 48 // float32 (4B)
)

type SpriteNode struct {
	Spritesheet    *SpritesheetNode
	Format         wgpu.TextureFormat
	UniformBuffer  *wgpu.Buffer
	SpriteBuffer   *wgpu.Buffer
	InstanceBuffer *wgpu.Buffer
	InstanceBytes  []byte // staging (len = INSTANCE_STRIDE * cap)
	InstanceCap    int

	IsScreenSpace bool

	Sprites      []SpriteInstance // all sprites
	Visible      []SpriteInstance // subset of all sprites visible on the screen
	VisibleCount int              // length of Visible

	LoadOp    wgpu.LoadOp
	Pipeline  *wgpu.RenderPipeline
	BindGroup *wgpu.BindGroup

	TargetFB *FrameBufferNode
}

type SpriteInstance struct {
	Position [2]float32
	Size     [2]float32
	Scale    [2]float32
	Tint     [4]float32
	SpriteID uint32
	Opacity  float32
	Rotation float32
	Id       uint32
}

func (s *SpriteNode) Init(c *State) error {
	// 4x4 matrix with 4 bytes per float32, times 2 matrices (view, projection)
	buf := [64 * 2]byte{}

	uniformBuffer, err := c.Device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "SpriteTransform",
		Contents: wgpu.ToBytes(buf[:]),
		Usage:    wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return err
	}

	s.UniformBuffer = uniformBuffer

	// Pack into std430-like struct (4*float*? + vec2 + vec2 → 32 bytes). We'll just write tightly as 8 floats.
	const FLOATS_PER_DESC = 8 // 8 float32s

	f32 := make([]float32, FLOATS_PER_DESC*len(s.Spritesheet.Spritetable.Descs))

	for i, d := range s.Spritesheet.Spritetable.Descs {
		base := i * 8

		f32[base+0] = d.UvOrigin[0]
		f32[base+1] = d.UvOrigin[1]
		f32[base+2] = d.UvSpan[0]
		f32[base+3] = d.UvSpan[1]
		f32[base+4] = float32(d.FrameSize[0])
		f32[base+5] = float32(d.FrameSize[1])
		f32[base+6] = d.CenterOffset[0]
		f32[base+7] = d.CenterOffset[1]
	}

	// create buffer for sprite uv lookup
	spriteBuf, err := c.Device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "srite desc table",
		Contents: wgpu.ToBytes(f32[:]),
		Usage:    wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return err
	}

	s.SpriteBuffer = spriteBuf

	// --- Instance buffer (growable) ---
	const instanceCap = 1024

	instanceBytes := make([]byte, INSTANCE_STRIDE*instanceCap)

	instanceBuf, err := c.Device.CreateBufferInit(&wgpu.BufferInitDescriptor{
		Label:    "sprite instances",
		Contents: instanceBytes,
		Usage:    wgpu.BufferUsageVertex | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return err
	}

	s.InstanceBuffer = instanceBuf
	s.InstanceCap = instanceCap
	s.InstanceBytes = instanceBytes

	bindGroupLayout, err := c.Device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageVertex,
				Buffer: wgpu.BufferBindingLayout{
					Type: wgpu.BufferBindingTypeUniform,
				},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageFragment,
				Sampler: wgpu.SamplerBindingLayout{
					Type: wgpu.SamplerBindingTypeFiltering,
				},
			},

			{
				Binding:    2,
				Visibility: wgpu.ShaderStageFragment,
				Texture: wgpu.TextureBindingLayout{
					SampleType:    wgpu.TextureSampleTypeFloat,
					ViewDimension: wgpu.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
			{
				Binding:    3,
				Visibility: wgpu.ShaderStageVertex,
				Buffer: wgpu.BufferBindingLayout{
					Type: wgpu.BufferBindingTypeReadOnlyStorage,
				},
			},
		},
	})
	if err != nil {
		return err
	}

	bindGroup, err := c.Device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding: 0,
				Buffer:  uniformBuffer,
				Offset:  0,
				Size:    wgpu.WholeSize, // whole buffer
			},
			{
				Binding: 1,
				Sampler: s.Spritesheet.ColorTexture.Sampler,
			},
			{
				Binding:     2,
				TextureView: s.Spritesheet.ColorTexture.View,
			},
			{
				Binding: 3,
				Buffer:  spriteBuf,
				Offset:  0,
				Size:    wgpu.WholeSize, // whole buffer
			},
		},
	})
	if err != nil {
		return err
	}

	s.BindGroup = bindGroup

	// --- pipeline layout (order matters: [tile, atlas]) ---
	pipelineLayout, err := c.Device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		return err
	}

	spriteShader, err := c.Device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "sprite.wgsl",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{
			Code: spriteWGSL,
		},
	})
	if err != nil {
		fmt.Println("shader compilation failed:", err)
		return err
	}

	defer spriteShader.Release()

	fmt.Println("c.Config.Format:", c.Config.Format)

	pipeline, err := c.Device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     spriteShader,
			EntryPoint: "vs_main",
			Buffers: []wgpu.VertexBufferLayout{
				// per-instance vertex buffer layout
				{
					ArrayStride: uint64(INSTANCE_STRIDE),
					StepMode:    wgpu.VertexStepModeInstance,
					Attributes: []wgpu.VertexAttribute{
						{ShaderLocation: 0, Offset: uint64(OFF_POS), Format: wgpu.VertexFormatFloat32x2},
						{ShaderLocation: 1, Offset: uint64(OFF_SIZE), Format: wgpu.VertexFormatFloat32x2},
						{ShaderLocation: 2, Offset: uint64(OFF_SCALE), Format: wgpu.VertexFormatFloat32x2},
						{ShaderLocation: 3, Offset: uint64(OFF_TINT), Format: wgpu.VertexFormatFloat32x4},
						{ShaderLocation: 4, Offset: uint64(OFF_SPRITEID), Format: wgpu.VertexFormatUint32},
						{ShaderLocation: 5, Offset: uint64(OFF_OPACITY), Format: wgpu.VertexFormatFloat32},
						{ShaderLocation: 6, Offset: uint64(OFF_ROT), Format: wgpu.VertexFormatFloat32},
					},
				},
			},
		},
		Fragment: &wgpu.FragmentState{
			Module:     spriteShader,
			EntryPoint: "fs_main",
			Targets: []wgpu.ColorTargetState{
				{
					Format:    c.Config.Format,
					WriteMask: wgpu.ColorWriteMaskAll,
					Blend: &wgpu.BlendState{
						Color: wgpu.BlendComponent{
							SrcFactor: wgpu.BlendFactorSrcAlpha,
							DstFactor: wgpu.BlendFactorOneMinusSrcAlpha,
						},
						Alpha: wgpu.BlendComponent{
							SrcFactor: wgpu.BlendFactorZero,
							DstFactor: wgpu.BlendFactorOne,
						},
					},
				},
			},
		},
		Primitive: wgpu.PrimitiveState{
			Topology:  wgpu.PrimitiveTopologyTriangleStrip,
			CullMode:  wgpu.CullModeNone,
			FrontFace: wgpu.FrontFaceCCW,
		},
		Multisample: wgpu.MultisampleState{
			Count: 1,
			Mask:  0xFFFFFFFF,
			// AlphaToCoverageEnabled: false,
		},
	})
	if err != nil {
		return err
	}

	s.Pipeline = pipeline

	return nil
}

func (s *SpriteNode) GetType() string {
	return "cobalt:sprite"
}

func (s *SpriteNode) IsEnabled() bool {
	return true
}

// view is the backing frame texture view that is created each frame
func (s *SpriteNode) OnRun(c *State, encoder *wgpu.CommandEncoder, view *wgpu.TextureView) error {
	s.Visible = s.Visible[:0]

	for i := range s.Sprites {
		sp := s.Sprites[i]

		d := s.Spritesheet.Spritetable.Descs[sp.SpriteID]
		// avoid sprite viewport culling when drawing in screen space mode (typically ui/hud layers)
		if !s.IsScreenSpace {
			sx := float32(d.FrameSize[0]) * sp.Size[0] * sp.Scale[0] * 0.5
			sy := float32(d.FrameSize[1]) * sp.Size[1] * sp.Scale[1] * 0.5

			rad := float32(math.Hypot(float64(sx), float64(sy)))
			x := sp.Position[0]
			y := sp.Position[1]

			if x+rad < float32(c.Viewport.position[0]) ||
				x-rad > float32(c.Viewport.position[0]+c.Viewport.width) ||
				y+rad < float32(c.Viewport.position[1]) ||
				y-rad > float32(c.Viewport.position[1]+c.Viewport.height) {
				continue
			}
		}

		s.Visible = append(s.Visible, sp)
	}

	s.VisibleCount = len(s.Visible)

	if s.VisibleCount < 1 {
		return nil
	}

	err := ensureCapacity(c, s, s.VisibleCount)
	if err != nil {
		return err
	}

	// pack instances into staging buffer
	for i := 0; i < s.VisibleCount; i++ {
		base := i * INSTANCE_STRIDE
		v := s.Visible[i]

		putF32(s.InstanceBytes, base+OFF_POS+0, v.Position[0])
		putF32(s.InstanceBytes, base+OFF_POS+4, v.Position[1])

		putF32(s.InstanceBytes, base+OFF_SIZE+0, v.Size[0])
		putF32(s.InstanceBytes, base+OFF_SIZE+4, v.Size[1])

		putF32(s.InstanceBytes, base+OFF_SCALE+0, v.Scale[0])
		putF32(s.InstanceBytes, base+OFF_SCALE+4, v.Scale[1])

		putF32(s.InstanceBytes, base+OFF_TINT+0, v.Tint[0])
		putF32(s.InstanceBytes, base+OFF_TINT+4, v.Tint[1])
		putF32(s.InstanceBytes, base+OFF_TINT+8, v.Tint[2])
		putF32(s.InstanceBytes, base+OFF_TINT+12, v.Tint[3])

		binary.LittleEndian.PutUint32(s.InstanceBytes[base+OFF_SPRITEID:], v.SpriteID)
		putF32(s.InstanceBytes, base+OFF_OPACITY, v.Opacity)
		putF32(s.InstanceBytes, base+OFF_ROT, v.Rotation)
	}

	err2 := c.Queue.WriteBuffer(s.InstanceBuffer, 0, s.InstanceBytes)

	if err2 != nil {
		return err2
	}

	// if no outview is provided, blit to the device's default frame texture
	v := view


	if s.TargetFB != nil {
		v = s.TargetFB.Material.View
		//fmt.Println("rendering to framebuffer", s.TargetFB)
	}


	// on the first render, we should clear the color attachment.
	// otherwise load it, so multiple sprite passes can build up data in the color and emissive textures
	renderPass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "sprite renderpass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       v,
				ClearValue: wgpu.Color{R: 0.0, G: 0.0, B: 0.0, A: 1.0},
				LoadOp:     s.LoadOp,
				StoreOp:    wgpu.StoreOpStore,
			},
		},
	})

	renderPass.SetPipeline(s.Pipeline)
	renderPass.SetBindGroup(0, s.BindGroup, nil)
	renderPass.SetVertexBuffer(0, s.InstanceBuffer, 0, wgpu.WholeSize)
	renderPass.Draw(4, uint32(s.VisibleCount), 0, 0) // triangle strip, 4 verts per instance

	defer renderPass.Release()
	return renderPass.End()
}

func (s *SpriteNode) OnDestroy(c *State) error {
	s.UniformBuffer.Destroy()
	s.SpriteBuffer.Destroy()
	s.InstanceBuffer.Destroy()

	s.Pipeline.Release()
	s.BindGroup.Release()

	return nil
}

func (s *SpriteNode) OnViewportPosition(c *State) error {
	return writeSpriteBuffer(c, s)
}

func (s *SpriteNode) OnResize(c *State) error {
	return writeSpriteBuffer(c, s)
}

func ensureCapacity(c *State, s *SpriteNode, nInstances int) error {
	if nInstances <= s.InstanceCap {
		return nil
	}
	// TODO
	/*
		let newCap = instanceCap
		if (newCap === 0) newCap = 1024

		while (newCap < nInstances) newCap *= 2

		node.data.instanceBuf.destroy()
		node.data.instanceBuf = cobalt.device.createBuffer({
		    size: INSTANCE_STRIDE * newCap,
		    usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST,
		})

		node.data.instanceStaging = new ArrayBuffer(INSTANCE_STRIDE * newCap)
		node.data.instanceView = new DataView(node.data.instanceStaging)
		node.data.instanceCap = newCap
	*/
	return nil
}

func writeSpriteBuffer(c *State, s *SpriteNode) error {
	vp := c.Viewport

	gameW := vp.width / vp.Zoom
	gameH := vp.height / vp.Zoom

	projection := mgl32.Ortho(0, float32(gameW), float32(gameH), 0, -10.0, 10.0)

	var tmpVec3 [3]float32

	// Camera position
	if s.IsScreenSpace {
		tmpVec3 = [3]float32{0, 0, 0}
	} else {
		// JS used round-half-up symmetric; math.Round in Go is half away from zero (good here).
		tmpVec3 = [3]float32{
			float32(-math.Round(float64(vp.position[0]))),
			float32(-math.Round(float64(vp.position[1]))),
			0,
		}
		// Alternative (no rounding):
		// tmpVec3 = [3]float32{-vp.Position[0], -vp.Position[1], 0}
	}

	view := mgl32.Translate3D(tmpVec3[0], tmpVec3[1], tmpVec3[2])

	// Upload (two back-to-back 4x4 matrices, 64 bytes each)
	err := c.Queue.WriteBuffer(s.UniformBuffer, 0, wgpu.ToBytes(view[:]))
	if err != nil {
		fmt.Println("err1", err)
		return err
	}

	err2 := c.Queue.WriteBuffer(s.UniformBuffer, 64, wgpu.ToBytes(projection[:]))
	if err2 != nil {
		fmt.Println("err2", err2)
		return err
	}
	return nil
}

// returns a unique identifier for the created sprite
func (s *SpriteNode) AddSprite(c *State, name string, position [2]float32, scale [2]float32, tint [4]float32, opacity float32, rotation float32) uint32 {
	spriteID := uint32(s.Spritesheet.IdByName[name])
	id := uint32(1 + rand.IntN(999999999))

	testSprite := SpriteInstance{
		Position: position,
		Size:     [2]float32{1, 1},
		Scale:    scale,
		Tint:     tint,
		SpriteID: spriteID,
		Opacity:  opacity,
		Rotation: rotation,
		Id:       id,
	}

	s.Sprites = append(s.Sprites, testSprite)

	return id
}

func (s *SpriteNode) RemoveSprite(c *State, spriteId uint32) {

	for i, sp := range s.Sprites {
		if sp.Id == spriteId {
            s.Sprites = slices.Delete(s.Sprites, i, i+1)
			return
		}
	}
}

// TODO: implement these remaining public API functions:
/*
// remove all sprites
export function clear(cobalt, renderPass) {
    renderPass.data.sprites.length = 0
}

export function setSpriteName(cobalt, renderPass, id, name) {
    const sprite = renderPass.data.sprites.find((s) => s.id === id)

    if (!sprite) return

    const { idByName } = renderPass.refs.spritesheet.data

    sprite.spriteID = idByName.get(name)
}

export function setSpritePosition(cobalt, renderPass, id, position) {
    const sprite = renderPass.data.sprites.find((s) => s.id === id)
    if (!sprite) return

    vec2.copy(position, sprite.position)
}

export function setSpriteTint(cobalt, renderPass, id, tint) {
    const sprite = renderPass.data.sprites.find((s) => s.id === id)
    if (!sprite) return

    vec4.copy(tint, sprite.tint)
}

export function setSpriteOpacity(cobalt, renderPass, id, opacity) {
    const sprite = renderPass.data.sprites.find((s) => s.id === id)
    if (!sprite) return

    sprite.opacity = opacity
}

export function setSpriteRotation(cobalt, renderPass, id, rotation) {
    const sprite = renderPass.data.sprites.find((s) => s.id === id)
    if (!sprite) return

    sprite.rotation = rotation
}

export function setSpriteScale(cobalt, renderPass, id, scale) {
    const sprite = renderPass.data.sprites.find((s) => s.id === id)
    if (!sprite) return

    vec2.copy(scale, sprite.scale)
}
*/
