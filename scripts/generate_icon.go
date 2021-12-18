package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"log"
	"os"
	"strconv"

	"github.com/J-Siu/go-helper"
	"github.com/jackmordaunt/icns"
)

/*
From https://github.com/J-Siu/go-png2ico to encode png to ico.

The MIT License
Copyright (c) 2021 John Siu
Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

*/

// ICO structire
type ICO struct {
	file string
	fh   *os.File
}

// PNG structure
type PNG struct {
	file   string // filename
	fh     *os.File
	height uint8
	width  uint8
	depth  uint16 // bit/pixel
	size   uint32
	offset uint32
	isPNG  bool
	buf    []byte
}

// Open : open PNG file
func (png *PNG) Open(file string) error {
	helper.DebugLog("PNG:Open:", file)

	var e error
	var n int

	png.file = file
	png.isPNG = false

	png.fh, e = os.Open(file)
	if e != nil {
		return e
	}

	/*
		25byte PNG header - BigEndian
		00:	89 50 4e 47 0d 0a 1a 0a // 8byte - magic number
		IHDR chunk
		08:	xx xx xx xx // 4byte - chunk length
		12:	49 48 44 52 // 4byte - chunk type(IHDR)
		16:	xx xx xx xx // 4byte - width
		20:	xx xx xx xx // 4byte - height
		24:	xx          // 1byte - bit depth (bit/pixel)
	*/
	headerLen := 25
	header := make([]byte, headerLen)
	n, e = png.fh.Read(header)
	if e != nil {
		return e
	}
	helper.DebugLog("PNG:Open:Header:", hex.EncodeToString(header), "(", n, ")")

	// 8byte header[0:8] - magic number
	magic := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if bytes.Equal(magic[:], header[:8]) {
		helper.DebugLog("PNG:Open: Found PNG magic")
	} else {
		return errors.New("Not PNG")
	}

	// 4byte header[8:12] - chunk length - skipped

	// 4byte header[12:16] - chunk type IHDR
	if bytes.Equal([]byte("IHDR"), header[12:16]) {
		helper.DebugLog("PNG:Open: Found IHDR chunk")
	} else {
		return errors.New("PNG no IHDR chunk")
	}

	// It is PNG
	png.isPNG = true

	// 4byte header[16:20] - width
	width := binary.BigEndian.Uint32(header[16:20])
	helper.DebugLog("PNG:Open:width:", width)

	// 4byte header[20:24] - height
	height := binary.BigEndian.Uint32(header[20:24])
	helper.DebugLog("PNG:Open:height:", height)

	if width <= 256 && height <= 256 {
		// ICO format use 0 for 256px
		if width == 256 {
			width = 0
		}
		if height == 256 {
			height = 0
		}
	} else {
		return errors.New(png.file + "(" + strconv.FormatUint(uint64(width), 10) + "x" + strconv.FormatUint(uint64(height), 10) + "): Width and height cannot be larger than 256.")
	}
	png.width = uint8(width)
	png.height = uint8(height)

	// 1byte header[25] - color depth
	png.depth = uint16(uint8(header[24]))
	helper.DebugLog("PNG:Open:depth:", png.depth)

	stat, _ := os.Stat(file)
	png.size = uint32(stat.Size())
	helper.DebugLog("PNG:Open:size:", png.size)

	// Pass all check, create PNG struct
	helper.DebugLog("PNG:Open:png:", *png)

	return nil
}

// Read : read PNG file
func (png *PNG) Read() error {
	helper.DebugLog("PNG:Read:", png.file)

	var e error
	var n int
	var n64 int64

	n64, e = png.fh.Seek(0, 0)
	helper.ErrCheck(e)
	helper.DebugLog("PNG:Read:Seek:", n64)

	png.buf = make([]byte, png.size)
	n, e = png.fh.Read(png.buf)
	helper.DebugLog("PNG:Read:byte:", n)

	return e
}

// Open : open ICO filehandle
func (ico *ICO) Open(file string) error {
	var e error
	helper.DebugLog("ICO:Open:", file)
	ico.fh, e = os.Create(file)
	return (e)
}

// Write : write ICO
func (ico *ICO) Write(b *[]byte) error {
	var e error
	var n int
	n, e = ico.fh.Write(*b)
	helper.DebugLog("ICO:Write:byte:", n)
	return (e)
}

// ICONDIR - return ICONDIR byte array
func (ico *ICO) ICONDIR(num uint16) *[]byte {
	/*
		6byte ICONDIR - LittleEndian
		00:   00 00 // 2byte, must be 0
		02:   01 00 // 2byte, 1 for ICO
		04:   xx xx // 2byte, img number
	*/

	b := []byte{0, 0, 1, 0, 0, 0}
	binary.LittleEndian.PutUint16(b[4:6], num)
	helper.DebugLog("ICO:ICONDIR:", hex.EncodeToString(b))
	return &b
}

// ICONDIRENTRY - return ICONDIRENTRY byte array
func (png *PNG) ICONDIRENTRY() *[]byte {
	helper.DebugLog("PNG:ICONDIRENTRY:png:", *png)
	/*
		16byte ICONDIRENTRY - LittleEndian
		00:   xx    // 1byte, width
		01:   xx    // 1byte, height
		02:   00    // 1byte, color palette number, 0 for PNG
		03:   00    // 1byte, reserved, always 0
		04:   00 00 // 2byte, color planes, 0 for PNG
		06:   xx xx // 2byte, color depth
		08:   xx xx xx xx // 4byte, image size
		12:   xx xx xx xx // 4byte, image offset
	*/

	b := make([]byte, 16)

	copy(b[0:6], []byte{png.width, png.height, 0, 0, 0, 0})
	binary.LittleEndian.PutUint16(b[6:8], png.depth)
	binary.LittleEndian.PutUint32(b[8:12], png.size)
	binary.LittleEndian.PutUint32(b[12:16], png.offset)
	helper.DebugLog("PNG:ICONDIRENTRY:", hex.EncodeToString(b))

	return &b
}

func createIcoFromPng(filein, fileout string) {
	// Get and calculate all PNGs info
	pngs := []*PNG{}
	pngc := 1
	var pngTotalSize uint32 = 0
	var LenICONDIR uint32 = 6
	var LenICONDIRENTRY uint32 = 16
	var LenAllICONDIRENTRY uint32 = LenICONDIRENTRY * uint32(pngc)
	for i := 0; i < pngc; i++ {
		png := new(PNG)
		helper.ErrCheck(png.Open(filein))
		// offset = len(ICONDIR) + len(all ICONDIRENTRY) + len(all PNG before current one)
		png.offset = LenICONDIR + LenAllICONDIRENTRY + pngTotalSize
		pngs = append(pngs, png)
		pngTotalSize += png.size
	}

	// Open ICON
	ico := new(ICO)
	helper.ErrCheck(ico.Open(fileout))
	helper.ErrCheck(ico.Write(ico.ICONDIR(uint16(pngc))))
	// Write ICONDIRENTRY
	for i := 0; i < pngc; i++ {
		helper.ErrCheck(ico.Write(pngs[i].ICONDIRENTRY()))
	}
	// Copy PNG
	for i := 0; i < pngc; i++ {
		helper.ErrCheck(pngs[i].Read())
		helper.ErrCheck(ico.Write(&pngs[i].buf))
	}
}

func createIcnsFromPng(filein, fileout string) {
	pngf, err := os.Open(filein)
	if err != nil {
		log.Fatalf("opening source image: %v", err)
	}
	defer pngf.Close()
	srcImg, _, err := image.Decode(pngf)
	if err != nil {
		log.Fatalf("decoding source image: %v", err)
	}
	dest, err := os.Create(fileout)
	if err != nil {
		log.Fatalf("opening destination file: %v", err)
	}
	defer dest.Close()
	if err := icns.Encode(dest, srcImg); err != nil {
		log.Fatalf("encoding icns: %v", err)
	}
}

func main() {
	createIcoFromPng("res/app.png", "res/win/app.ico")
	createIcnsFromPng("res/app.png", "res/mac/app.icns")
}
