package main

import (
	"encoding/json"
	"log"
	"os"
)

type Layer struct {
	Name    string
	Type    string
	Cells   []uint16
	Visible bool
}

type Tileset struct {
	Cols uint16
	Rows uint16
}

type Level struct {
	Cols       uint16
	Rows       uint16
	Name       string
	LayerOrder []uint32
	Layers     map[uint32]Layer
	Tileset    Tileset
}

type MapFile struct {
	Level   Level
	Version uint8
}

func LoadMap(mapPath string) MapFile {
	var m MapFile

	b, err := os.ReadFile(mapPath)

	if err != nil {
		log.Fatal(err)
	}

	json.Unmarshal(b, &m)
	return m
}
