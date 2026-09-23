package discordbot

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func renderWelcomePNG(displayName string, memberCount int) []byte {
	const w, h = 1000, 320
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h-1)
		r := uint8(15 + t*29)
		g := uint8(32 + t*51)
		b := uint8(39 + t*61)
		draw.Draw(img, image.Rect(0, y, w, y+1), &image.Uniform{C: color.RGBA{R: r, G: g, B: b, A: 255}}, image.Point{}, draw.Src)
	}
	draw.Draw(img, image.Rect(0, 0, 8, h), &image.Uniform{C: color.RGBA{R: 0x1A, G: 0xBC, B: 0x9C, A: 255}}, image.Point{}, draw.Src)
	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.White), Face: basicfont.Face7x13}
	d.Dot = fixed.P(40, 80)
	d.DrawString("WELCOME")
	d.Src = image.NewUniform(color.RGBA{R: 0x1A, G: 0xBC, B: 0x9C, A: 255})
	d.Dot = fixed.P(40, 140)
	name := displayName
	if len(name) > 40 {
		name = name[:40]
	}
	d.DrawString(name)
	d.Src = image.NewUniform(color.RGBA{R: 180, G: 200, B: 210, A: 255})
	d.Dot = fixed.P(40, 200)
	d.DrawString("Skillissue.ai  ·  member #" + strconv.Itoa(memberCount))
	d.Dot = fixed.P(40, 260)
	d.DrawString("Read the rules, then Verify. Staff never ask for passwords or OTP.")
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
