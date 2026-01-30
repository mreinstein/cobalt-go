package main

import (
	//"fmt"
	"math"
	//"os"
	"runtime"
	"strconv"
	//"strings"
	"time"

	"github.com/mreinstein/cobalt-go/cobalt"

	"github.com/cogentcore/webgpu/wgpu"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type snap struct {
	axes    []float32
	buttons []byte // store as bytes so it works for both raw and gamepad
}

func main() {
	runtime.LockOSThread()

	// glfw.WindowHint(glfw.CocoaRetinaFramebuffer, glfw.False)

	if err := glfw.Init(); err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)

	window, err := glfw.CreateWindow(1440, 810, "go + webgpu + glfw", nil, nil)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()

	c, err := cobalt.Init(window, 1440, 810)
	defer cobalt.Reset(c)

	if err != nil {
		panic(err)
	}

	c.Viewport.Zoom = 1.0

	/*
	       fb := &cobalt.FrameBufferNode{
	       	Label: "fb test",
	   		Format:   c.Config.Format,
	   		//GPUTextureUsage.RENDER_ATTACHMENT | GPUTextureUsage.STORAGE_BINDING | GPUTextureUsage.TEXTURE_BINDING
	   		Usage:    wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst | wgpu.TextureUsageRenderAttachment,
	   		MipCount: 1,
	       }

	       _ = fb.Init(c)
	       c.Nodes = append(c.Nodes, fb)
	*/

	ta := &cobalt.TileAtlasNode{
		TexturePath: "./assets/tileset.png",
		Format:      wgpu.TextureFormatRGBA8Unorm,
		TileScale:   1.0,
		TileSize:    16,
	}

	_ = ta.Init(c)
	c.Nodes = append(c.Nodes, ta)

	for i := 0; i < 7; i++ {
		tl := &cobalt.TileLayerNode{
			TileAtlas:   ta,
			TexturePath: "./assets/layer" + strconv.Itoa(i) + ".png",
			Format:      wgpu.TextureFormatRGBA8Unorm,
			ScrollScale: 1.0,
		}

		_ = tl.Init(c)
		c.Nodes = append(c.Nodes, tl)
	}

	ss := &cobalt.SpritesheetNode{
		SpritesheetJsonPath: "./assets/spritesheet.json",
		ColorTexturePath:    "./assets/spritesheet.png",
		Format:              wgpu.TextureFormatRGBA8Unorm,
	}

	err2 := ss.Init(c)
	if err2 != nil {
		panic(err2)
	}
	c.Nodes = append(c.Nodes, ss)

	sn := &cobalt.SpriteNode{
		Spritesheet:   ss,
		Format:        wgpu.TextureFormatRGBA8Unorm,
		IsScreenSpace: false,
		LoadOp:        wgpu.LoadOpLoad,
		//TargetFB: fb,
	}

	_ = sn.Init(c)

	c.Nodes = append(c.Nodes, sn)

	// TODO: allow passing spriteId into the function. I suspect most or all
	// of the sprites will be created on the node side.
	//
	// returns a unique spriteId that can be used to modify it later
	sid := sn.AddSprite(c, "hero_idle_look_forward-0.png", [2]float32{2400.0, 1850.0}, [2]float32{1.0, 1.0}, [4]float32{0.0, 0.0, 1.0, 0.0}, 1.0, 0.0)

	/*
		blit := &cobalt.BlitNode{
			SourceFb: fb,
		}

		err = blit.Init(c)

		if err != nil {
			panic(err)
		}
		c.Nodes = append(c.Nodes, blit)
	*/

	window.SetSizeCallback(func(w *glfw.Window, width, height int) {
		updateWindowSize(w, c, width, height)
	})

	updateWindowSize(window, c, 1440, 810)

	cobalt.SetViewportPosition(c, [2]int{2400, 1800})

	/*
		prev := map[glfw.Joystick]snap{}

		for jid := glfw.Joystick1; jid <= glfw.JoystickLast; jid++ {
			js := glfw.Joystick(jid)
			if js.Present() {
				fmt.Printf("Joystick %d connected: %s\n", jid, js.GetName())

				if js.IsGamepad() {
					fmt.Printf(" -> Detected as gamepad: %s\n", js.GetGamepadName())
					state := js.GetGamepadState()
					if state != nil {
						for i, axis := range state.Axes {
							fmt.Printf("   Axis %d: %.3f\n", i, axis)
						}
						for i, button := range state.Buttons {
							fmt.Printf("   Button %d: %v\n", i, button)
						}
					}
				} else {
					axes := js.GetAxes()
					buttons := js.GetButtons()
					fmt.Printf(" -> Not a standard gamepad (Axes=%d, Buttons=%d)\n",
						len(axes), len(buttons))
				}
			}
		}
	*/

	target := time.Second / 120 // cap at ~120 FPS

	for !window.ShouldClose() {
		start := time.Now()

		glfw.PollEvents()

		// hide the cursor when it's over the game window, otherwise show it normally:
		// glfwSetInputMode(window, GLFW_CURSOR, GLFW_CURSOR_HIDDEN)

		// https://www.glfw.org/docs/latest/input_guide.html
		// https://www.glfw.org/docs/latest/input_guide.html#gamepad_mapping
		/*
		    here's what I'm thinking for input polling:

		    foreach action
		      pressed = false
		      if keyboard key is pressed
		          pressed = true

		      foreach joystick
		          if active and a gamepad
		             if key is pressed
		               pressed = true
		               goto next action

		   return action states
		*/

		// https://github.com/go-gl/glfw/blob/master/v3.3/glfw/input.go
		if sid > 0 && window.GetKey(glfw.KeyE) == glfw.Press {
			sn.RemoveSprite(c, sid)
			sid = 0
		}

		// t0 := time.Now()
		err := cobalt.Draw(c)
		if err != nil {
			panic(err)
		}

		if sleep := target - time.Since(start); sleep > 0 {
			time.Sleep(sleep)
		}

		/*
			for jid := glfw.Joystick1; jid <= glfw.JoystickLast; jid++ {
				js := glfw.Joystick(jid)
				if !js.Present() {
					delete(prev, jid)
					continue
				}

				if js.IsGamepad() {
					st := js.GetGamepadState()
					if st == nil {
						continue
					}
					axes := make([]float32, len(st.Axes))
					copy(axes, st.Axes[:])
					btns := make([]byte, len(st.Buttons))
					for i, b := range st.Buttons {
						btns[i] = byte(b)
					}

					if diffAndPrintGamepad(js, axes, btns, prev[jid]) {
						prev[jid] = snap{axes: axes, buttons: btns}
					}
				} else {
					axes := js.GetAxes()
					btnActions := js.GetButtons()
					btns := make([]byte, len(btnActions))
					for i, b := range btnActions {
						btns[i] = byte(b)
					}

					if diffAndPrintRaw(js, axes, btns, prev[jid]) {
						// copy axes so our snapshot doesn't alias GLFW's buffer
						axCopy := append([]float32(nil), axes...)
						prev[jid] = snap{axes: axCopy, buttons: append([]byte(nil), btns...)}
					}
				}
			}

			dt := time.Since(t0)

			if 5 == 23 {
				fmt.Println("dt:", dt)
			}


			if err != nil {
				fmt.Println("error occured while rendering:", err)

				errstr := err.Error()
				switch {
				case strings.Contains(errstr, "Surface timed out"): // do nothing
				case strings.Contains(errstr, "Surface is outdated"): // do nothing
				case strings.Contains(errstr, "Surface was lost"): // do nothing
				default:
					panic(err)
				}
			}
		*/
	}
}

func updateWindowSize(w *glfw.Window, c *cobalt.State, width, height int) {

	/*
	     16:9    16:10     3:2
	   480x270,  480x300, 480x320
	             432x270, 405x270
	           1792x1120
	*/

	width, height = w.GetSize() // logical points
	idealWidth := 480.0

	// scaleFactor must be an integer because we get weird texture artifacts like blurring/shimmering/uneven
	// lines when trying to render at certain float scale values (e.g., 3.0145833333333334)
	scaleFactor := math.Round(float64(width) / idealWidth)

	if scaleFactor == 0 {
		scaleFactor = 1
	}

	gameWidth := math.Round(float64(width) / scaleFactor)
	gameHeight := math.Round(float64(height) / scaleFactor)

	// renderer.canvasScale = scaleFactor
	// c.Viewport.Zoom = int(scaleFactor)

	cobalt.SetViewportDimensions(c, int(gameWidth), int(gameHeight))
}

/*
func diffAndPrintGamepad(js glfw.Joystick, axes []float32, btns []byte, prev snap) bool {
	changed := false
	name := js.GetGamepadName()

	for i, v := range axes {
		if i >= len(prev.axes) || fchanged(prev.axes[i], v) {
			fmt.Printf("[%s] %s = % .3f\n", name, axisName(i), v)
			changed = true
		}
	}
	for i, b := range btns {
		var pb byte
		if i < len(prev.buttons) {
			pb = prev.buttons[i]
		}
		if pb != b {
			fmt.Printf("[%s] %s = %v\n", name, buttonName(i), b == byte(glfw.Press))
			changed = true
		}
	}
	return changed
}

func diffAndPrintRaw(js glfw.Joystick, axes []float32, btns []byte, prev snap) bool {
	changed := false
	name := js.GetName()

	for i, v := range axes {
		if i >= len(prev.axes) || fchanged(prev.axes[i], v) {
			fmt.Printf("[%s] Axis %d = % .3f\n", name, i, v)
			changed = true
		}
	}
	for i, b := range btns {
		var pb byte
		if i < len(prev.buttons) {
			pb = prev.buttons[i]
		}
		if pb != b {
			fmt.Printf("[%s] Button %d = %v\n", name, i, b == byte(glfw.Press))
			changed = true
		}
	}
	return changed
}

func fchanged(a, b float32) bool { return math.Abs(float64(a-b)) > 0.03 }

// GLFW's standard gamepad mapping names (indices are fixed by GLFW)
func axisName(i int) string {
	switch i {
	case 0:
		return "LeftX"
	case 1:
		return "LeftY"
	case 2:
		return "RightX"
	case 3:
		return "RightY"
	case 4:
		return "L2"
	case 5:
		return "R2"
	default:
		return fmt.Sprintf("Axis%d", i)
	}
}

func buttonName(i int) string {
	switch i {
	case 0:
		return "A/Cross"
	case 1:
		return "B/Circle"
	case 2:
		return "X/Square"
	case 3:
		return "Y/Triangle"
	case 4:
		return "LB/L1"
	case 5:
		return "RB/R1"
	case 6:
		return "Back/Select"
	case 7:
		return "Start/Options"
	case 8:
		return "Guide/PS"
	case 9:
		return "LeftThumb"
	case 10:
		return "RightThumb"
	case 11:
		return "DPadUp"
	case 12:
		return "DPadRight"
	case 13:
		return "DPadDown"
	case 14:
		return "DPadLeft"
	default:
		return fmt.Sprintf("Button%d", i)
	}
}
*/
