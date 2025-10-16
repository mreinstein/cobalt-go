package cobalt

import (
	"fmt"
	_ "embed"

	"github.com/cogentcore/webgpu/wgpu"
)

// blit a source texture into a destination texture

//go:embed node-blit.wgsl
var blitWGSL string

type BlitNode struct {
	SourceFb *FrameBufferNode
	OutView *wgpu.TextureView
	BindGroupLayout *wgpu.BindGroupLayout
	BindGroup *wgpu.BindGroup
	Pipeline *wgpu.RenderPipeline
}

func (t *BlitNode) Init(c *State) error {

	bindGroupLayout, err := c.Device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageFragment,
				Texture: wgpu.TextureBindingLayout{
					SampleType:    wgpu.TextureSampleTypeFloat,
					ViewDimension: wgpu.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
			{
				Binding:    1,
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

	t.BindGroupLayout = bindGroupLayout


	bindGroup, err := c.Device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding:     0,
				TextureView: t.SourceFb.Material.View,
			},
			{
				Binding: 1,
				Sampler: t.SourceFb.Material.Sampler,
			},
		},
	})

	if err != nil {
		return err
	}

	t.BindGroup = bindGroup


	pipelineLayout, err := c.Device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})

	if err != nil {
		return err
	}

	drawShader, err := c.Device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "node-blit.wgsl",
		WGSLDescriptor: &wgpu.ShaderModuleWGSLDescriptor{
			Code: blitWGSL,
		},
	})

	if err != nil {
		fmt.Println("shader compilation failed:", err)
		return err
	}

	defer drawShader.Release()

	pipeline, err := c.Device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "blit",
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     drawShader,
			EntryPoint: "vs_main",
			Buffers:    nil, // []wgpu.VertexBufferLayout{} if you have vertex buffers
		},
		Fragment: &wgpu.FragmentState{
			Module:     drawShader,
			EntryPoint: "fs_main",
			Targets: []wgpu.ColorTargetState{
				{
					Format: c.Config.Format,
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
			Topology: wgpu.PrimitiveTopologyTriangleList,
			CullMode:  wgpu.CullModeNone,
			FrontFace: wgpu.FrontFaceCCW,
		},
		// DepthStencil, Multisample as needed...
		Multisample: wgpu.MultisampleState{
			Count:                  1,          // <-- fix: 1 for no MSAA (or 4/8 if using MSAA)
			Mask:                   0xFFFFFFFF, // sample mask (all samples enabled)
			//AlphaToCoverageEnabled: false,
		},
	})

	if err != nil {
		return err
	}

	t.Pipeline = pipeline

	return nil
}

func (t *BlitNode) GetType() string {
	return "cobalt:blit"
}

func (t *BlitNode) IsEnabled() bool {
	return true
}

// view is the backing frame texture view that is created each frame
func (t *BlitNode) OnRun(c *State, encoder *wgpu.CommandEncoder, view *wgpu.TextureView) error {

	// if no outview is provided, blit to the device's default frame texture
	v := view

	if t.OutView != nil {
		v = t.OutView
	}

	renderPass := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "blit renderpass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       v,
				ClearValue: wgpu.Color{R: 0.0, G: 0.0, B: 1.0, A: 1.0},
				LoadOp:     wgpu.LoadOpLoad,
				StoreOp:    wgpu.StoreOpStore,
			},
		},
	})

	renderPass.SetPipeline(t.Pipeline)
	renderPass.SetBindGroup(0, t.BindGroup, nil)
	
	renderPass.Draw(3, 1, 0, 0) // triangle, 3 verts per instance

	defer renderPass.Release()
	return renderPass.End()

	return nil
}

func (t *BlitNode) OnDestroy(c *State) error {
	
	return nil
}

func (t *BlitNode) OnViewportPosition(c *State) error {
	return nil
}

func (t *BlitNode) OnResize(c *State) error {
	// re-build the bind group
	bindGroup, err := c.Device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: t.BindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{
				Binding:     0,
				TextureView: t.SourceFb.Material.View,
			},
			{
				Binding: 1,
				Sampler: t.SourceFb.Material.Sampler,
			},
		},
	})

	if err != nil {
		return err
	}

	t.BindGroup = bindGroup

	return nil
}
