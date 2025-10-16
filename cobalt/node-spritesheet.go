package cobalt

import (
	"encoding/json"
	"errors"
	//"fmt"
	"os"
	"sort"

	"github.com/cogentcore/webgpu/wgpu"
)

// ------------------------------ JSON types ------------------------------

type Doc struct {
	Frames map[string]Frame `json:"frames"`
	Meta   Meta             `json:"meta"`
}

type Meta struct {
	Size Size `json:"size"`
}

type Frame struct {
	Frame            Rect `json:"frame"`
	Rotated          bool `json:"rotated"`
	Trimmed          bool `json:"trimmed"`
	SpriteSourceSize Rect `json:"spriteSourceSize"`
	SourceSize       Size `json:"sourceSize"`
}

type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Size struct {
	W int `json:"w"`
	H int `json:"h"`
}

// Spritesheet types

type Desc struct {
	UvOrigin     [2]float32 // [offX, offY]
	UvSpan       [2]float32 // [spanX, spanY]
	FrameSize    [2]int     // [fw, fh]
	CenterOffset [2]float32 // [cx, cy]
}

type SpriteTable struct {
	Descs []Desc
	Names []string
}

type SpritesheetNode struct {
	SpritesheetJsonPath string
	ColorTexturePath    string
	Format              wgpu.TextureFormat
	ColorTexture        *Texture
	IdByName            map[string]int
	Spritetable         *SpriteTable
}

func (s *SpritesheetNode) Init(c *State) error {
	atlasMaterial, err := CreateTextureFromPath(c, "spritesheet", s.ColorTexturePath, s.Format)
	if err != nil {
		return err
	}

	rawJson, err := os.ReadFile(s.SpritesheetJsonPath)
	if err != nil {
		return err
	}

	var d Doc
	if err := json.Unmarshal(rawJson, &d); err != nil {
		return err
	}

	st, err := buildSpriteTableFromTexturePacker(&d)
	if err != nil {
		return err
	}

	// Map sprite name → ID
	idByName := make(map[string]int, len(st.Names))

	for i, name := range st.Names {
		idByName[name] = i
	}

	s.Spritetable = st
	s.ColorTexture = atlasMaterial
	s.IdByName = idByName
	return nil
}

func (s *SpritesheetNode) GetType() string {
	return "cobalt:spritesheet"
}

func (s *SpritesheetNode) IsEnabled() bool {
	return true
}

// view is the backing frame texture view that is created each frame
func (s *SpritesheetNode) OnRun(c *State, encoder *wgpu.CommandEncoder, view *wgpu.TextureView) error {
	return nil
}

func (s *SpritesheetNode) OnDestroy(c *State) error {
	if s.ColorTexture == nil {
		return nil
	}
	s.ColorTexture.Texture.Release()
	s.ColorTexture = nil
	return nil
}

func (s *SpritesheetNode) OnViewportPosition(c *State) error {
	return nil
}

func (t *SpritesheetNode) OnResize(c *State) error {
	return nil
}

// BuildSpriteTableFromTexturePacker mirrors the JS function.
// Assumes frames are not rotated (rotated=false in TexturePacker settings).
func buildSpriteTableFromTexturePacker(doc *Doc) (*SpriteTable, error) {
	atlasW := doc.Meta.Size.W
	atlasH := doc.Meta.Size.H
	if atlasW == 0 || atlasH == 0 {
		return nil, errors.New("invalid atlas size: width/height must be non-zero")
	}

	// Collect and sort names
	names := make([]string, 0, len(doc.Frames))
	for name := range doc.Frames {
		names = append(names, name)
	}
	sort.Strings(names)

	descs := make([]Desc, len(names))

	for i, name := range names {
		fr := doc.Frames[name]

		fx, fy := fr.Frame.X, fr.Frame.Y
		fw, fh := fr.Frame.W, fr.Frame.H

		offX := float32(fx) / float32(atlasW)
		offY := float32(fy) / float32(atlasH)
		spanX := float32(fw) / float32(atlasW)
		spanY := float32(fh) / float32(atlasH)

		sw, sh := fr.SourceSize.W, fr.SourceSize.H
		ox, oy := fr.SpriteSourceSize.X, fr.SpriteSourceSize.Y

		cx := float32(ox) + float32(fw)*0.5 - float32(sw)*0.5
		cy := float32(oy) + float32(fh)*0.5 - float32(sh)*0.5

		descs[i] = Desc{
			UvOrigin:     [2]float32{offX, offY},
			UvSpan:       [2]float32{spanX, spanY},
			FrameSize:    [2]int{fw, fh},
			CenterOffset: [2]float32{cx, cy},
		}
	}

	return &SpriteTable{Descs: descs, Names: names}, nil
}
