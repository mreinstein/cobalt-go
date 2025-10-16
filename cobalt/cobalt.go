package cobalt

import (
	"fmt"
	//"math"
	//"math/rand"
	"os"
	"runtime"
	//"strconv"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/cogentcore/webgpu/wgpuglfw"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// cobalt global data structure
type State struct {
	surface *wgpu.Surface
	adapter *wgpu.Adapter
	Device  *wgpu.Device
	Queue   *wgpu.Queue
	Config  *wgpu.SurfaceConfiguration

	// some nodes may need a reference to the default texture view (the frame backing)
	// this is generated each frame
	surfaceTexView *wgpu.TextureView

	nodeDefs map[string]NodeDefinition

	// used in the color attachments of renderpass
	// clearValue: { r: 0.0, g: 0.0, b: 0.0, a: 1.0 },

	// runnable nodes. ordering dictates render order (first to last)
	Nodes    []NodeDefinition
	Viewport Viewport
}

type NodeOptions struct {
	nodeType string
	payload  any
}

type NodeInstance struct {
	nodeType   string
	refs       []int
	options    map[string]string
	data       any
	enabled    bool           // when disabled, the node won't be run
	definition NodeDefinition // the implementation of the node
}

type NodeDefinition interface {
	Init(*State) error
	GetType() string
	IsEnabled() bool
	OnRun(*State, *wgpu.CommandEncoder, *wgpu.TextureView) error
	OnDestroy(*State) error
	OnViewportPosition(*State) error
	OnResize(*State) error
}

type Viewport struct {
	width    int
	height   int
	Zoom     int
	position [2]int // top-left corner of the viewport
}

// create and initialize a WebGPU renderer for a given glfw window
// returns the data structure containing all WebGPU related stuff
func Init(window *glfw.Window, viewportWidth int, viewportHeight int) (s *State, err error) {
	s = &State{}

	runtime.LockOSThread()

	switch os.Getenv("WGPU_LOG_LEVEL") {
	case "OFF":
		wgpu.SetLogLevel(wgpu.LogLevelOff)
	case "ERROR":
		wgpu.SetLogLevel(wgpu.LogLevelError)
	case "WARN":
		wgpu.SetLogLevel(wgpu.LogLevelWarn)
	case "INFO":
		wgpu.SetLogLevel(wgpu.LogLevelInfo)
	case "DEBUG":
		wgpu.SetLogLevel(wgpu.LogLevelDebug)
	case "TRACE":
		wgpu.SetLogLevel(wgpu.LogLevelTrace)
	}

	forceFallbackAdapter := os.Getenv("WGPU_FORCE_FALLBACK_ADAPTER") == "1"

	instance := wgpu.CreateInstance(nil)
	defer instance.Release()

	s.surface = instance.CreateSurface(wgpuglfw.GetSurfaceDescriptor(window))

	s.adapter, err = instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		ForceFallbackAdapter: forceFallbackAdapter,
		CompatibleSurface:    s.surface,
	})
	if err != nil {
		return s, err
	}
	defer s.adapter.Release()

	s.Device, err = s.adapter.RequestDevice(nil)
	if err != nil {
		return s, err
	}
	s.Queue = s.Device.GetQueue()

	caps := s.surface.GetCapabilities(s.adapter)

	width, height := window.GetSize()

	//fbw, fbh := window.GetFramebufferSize()

	s.Config = &wgpu.SurfaceConfiguration{
		Usage:       wgpu.TextureUsageRenderAttachment,
		Format:      wgpu.TextureFormatBGRA8Unorm, //wgpu.TextureFormatRGBA16Float
		Width:       uint32(width),
		Height:      uint32(height),
		PresentMode: wgpu.PresentModeFifo, //wgpu.PresentModeImmediate, // wgpu.PresentModeFifo,
		AlphaMode:   caps.AlphaModes[0],
	}

	s.surface.Configure(s.adapter, s.Device, s.Config)

	return s, nil
}

func DefineNode(c *State, nodeDefinition NodeDefinition) {
	key := nodeDefinition.GetType()
	c.nodeDefs[key] = nodeDefinition
}

func Draw(c *State) error {
	nextTexture, err := c.surface.GetCurrentTexture()
	if err != nil {
		return err
	}
	view, err := nextTexture.CreateView(nil)
	if err != nil {
		return err
	}
	c.surfaceTexView = view

	defer view.Release()

	commandEncoder, err := c.Device.CreateCommandEncoder(nil)
	if err != nil {
		return err
	}
	defer commandEncoder.Release()

	// run all enabled nodes
	for _, n := range c.Nodes {
		if n.IsEnabled() {
			n.OnRun(c, commandEncoder, view)
		}
	}

	cmdBuffer, err := commandEncoder.Finish(nil)
	if err != nil {
		return err
	}
	defer cmdBuffer.Release()

	c.Queue.Submit(cmdBuffer)
	c.surface.Present()

	return nil
}

func Reset(c *State) {
	for _, n := range c.Nodes {
		n.OnDestroy(c)
	}

	if c.Config != nil {
		c.Config = nil
	}
	if c.Queue != nil {
		c.Queue.Release()
		c.Queue = nil
	}
	if c.Device != nil {
		c.Device.Release()
		c.Device = nil
	}
	if c.surface != nil {
		c.surface.Release()
		c.surface = nil
	}
}

func SetViewportDimensions(c *State, width int, height int) {
	fmt.Println("set viewport dims!", width, height)

	if width > 0 && height > 0 {
		c.Config.Width = uint32(width)
		c.Config.Height = uint32(height)

		fmt.Println("resized to", width, height)
		c.surface.Configure(c.adapter, c.Device, c.Config)

		c.Viewport.width = width
		c.Viewport.height = height
		for _, n := range c.Nodes {
			n.OnResize(c)
		}
	}
}

func SetViewportPosition(c *State, pos [2]int) {
	fmt.Println("set viewport pos!", pos[0], pos[1], "zoom:", c.Viewport.Zoom)

	c.Viewport.position[0] = pos[0] - (c.Viewport.width / 2 / c.Viewport.Zoom)
	c.Viewport.position[1] = pos[1] - (c.Viewport.height / 2 / c.Viewport.Zoom)
	for _, n := range c.Nodes {
		n.OnViewportPosition(c)
	}
}

func InitNode(c *State, opts *NodeOptions) (*NodeInstance, error) {
	return nil, nil
}
