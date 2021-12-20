package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	g "github.com/AllenDang/giu"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var IS_DEV = false

var listOfVideos []string
var isFfmpegReady bool
var tmpbinPath string
var isConversionPreparing bool
var isCurrentlyConverting bool
var convertingHelperMsg string
var convertingFFmpegOutput bytes.Buffer

var sizeComboBoxLists = []string{
	"original",
	"480p",
	"720p",
	"1080p",
}
var sizeComboBoxIdx int32 = 0
var sizeToConvert = "original"

var resultingFilePrefix = "cvt-"

var (
	VERSION = "development"
)

func ffmpegOutputKwargs() ffmpeg.KwArgs {
	args := ffmpeg.KwArgs{
		"c:a": "copy",
		"c:v": "libx264",
	}

	switch sizeToConvert {
	case "480p":
		args["filter:v"] = "scale=trunc(oh*a/2)*2:480"
	case "720p":
		args["filter:v"] = "scale=trunc(oh*a/2)*2:720"
	case "1080p":
		args["filter:v"] = "scale=trunc(oh*a/2)*2:1080"
	}

	return args
}

func ffmpegCheck() {
	if _, err := exec.LookPath("ffmpeg"); nil != err {
		isFfmpegReady = false
	} else {
		isFfmpegReady = true
	}
}

func updateEnvPath() {
	tmpbinPath = path.Join(os.TempDir(), "bin")
	os.MkdirAll(tmpbinPath, os.FileMode(0755))

	os.Setenv("PATH", fmt.Sprintf("%s%c%s", os.Getenv("PATH"), os.PathListSeparator, tmpbinPath))
}

func downloadGzippedFfmpegFromURL(url string) error {
	resp, err := http.Get(url)
	if nil != err {
		return err
	}
	defer resp.Body.Close()

	gzReader, err := gzip.NewReader(resp.Body)
	if nil != err {
		return err
	}
	defer gzReader.Close()

	ffmpegExe := "ffmpeg"
	if "windows" == runtime.GOOS {
		ffmpegExe += ".exe"
	}

	f, err := os.OpenFile(path.Join(tmpbinPath, ffmpegExe), os.O_CREATE|os.O_RDWR, os.FileMode(0755))
	if nil != err {
		return err
	}

	if _, err := io.Copy(f, gzReader); nil != err {
		return err
	}

	return nil
}

func downloadFfmpeg() {
	switch runtime.GOOS {
	case "darwin":
		if "arm64" == runtime.GOARCH {
			downloadGzippedFfmpegFromURL("https://github.com/eugeneware/ffmpeg-static/releases/download/b4.4/darwin-arm64.gz")
		} else {
			downloadGzippedFfmpegFromURL("https://github.com/eugeneware/ffmpeg-static/releases/download/b4.4/darwin-x64.gz")
		}
	case "windows":
		downloadGzippedFfmpegFromURL("https://github.com/eugeneware/ffmpeg-static/releases/download/b4.4/win32-x64.gz")

	default:
		panic("not supported OS")
	}
}

func detectFileMimetype(filename string) (string, error) {
	f, err := os.Open(filename)
	if nil != err {
		return "", err
	}

	buf := make([]byte, 512)
	if _, err := f.Read(buf); nil != err {
		return "", err
	}

	return http.DetectContentType(buf), nil
}

func onClickConvert() {
	go convertVideo()
}

func convertVideo() {
	var wg sync.WaitGroup

	for _, videoFilename := range listOfVideos {
		wg.Add(1)
		isConversionPreparing = true
		convertingHelperMsg = ""

		go func(videoPath string, wg *sync.WaitGroup) {
			dirname := filepath.Dir(videoPath)
			filename := filepath.Base(videoPath)
			// fileext := filepath.Ext(videoPath)

			fileprefix := resultingFilePrefix
			if "original" != sizeToConvert {
				fileprefix += fmt.Sprintf("%s-", sizeToConvert)
			}

			convertedPath := fmt.Sprintf("%s/%s%s", dirname, fileprefix, filename)
			convertingHelperMsg = fmt.Sprintf("currently converting:\n %s\ndestination:\n %s", videoPath, convertedPath)

			isCurrentlyConverting = true
			err := ffmpeg.Input(videoPath).Output(convertedPath, ffmpegOutputKwargs()).OverWriteOutput().WithErrorOutput(&convertingFFmpegOutput).Run()
			if nil != err {
				convertingHelperMsg = err.Error()
			} else {
				convertingHelperMsg = fmt.Sprintf("finish conversion\ndestination:\n %s", convertedPath)
			}

			isConversionPreparing = false
			isCurrentlyConverting = false
			convertingFFmpegOutput.Reset()

			// deliberate sleep before finish
			time.Sleep(3 * time.Second)
			wg.Done()
		}(videoFilename, &wg)

		wg.Wait()
	}
}

func myLayouts() []g.Widget {
	var widgets []g.Widget

	if !isFfmpegReady {
		if IS_DEV {
			widgets = append(widgets, []g.Widget{
				g.Dummy(100, 80),
				g.Style().SetFontSize(16).To(
					g.Align(g.AlignCenter).To(
						g.Label(fmt.Sprintf("ffmpeg not found on PATH\nlookup path: %s", os.Getenv("PATH"))).Wrapped(true),
						g.Dummy(0, 30),
						g.Label(fmt.Sprintf("Currently downloading ffmpeg in %s", tmpbinPath)).Wrapped(true),
					),
				),
			}...)
		} else {
			widgets = append(widgets, []g.Widget{
				g.Style().SetFontSize(16).To(
					g.Align(g.AlignCenter).To(
						g.Label("initializing application... (download pre-requisite)").Wrapped(true),
					),
				),
			}...)
		}

		return widgets
	}

	if 0 == len(listOfVideos) {
		widgets = append(widgets, []g.Widget{
			g.Dummy(0, 180),
			g.Style().SetFontSize(20).To(
				g.Align(g.AlignCenter).To(g.Label("Drag and Drop your video files here!")),
			),
			g.Align(g.AlignCenter).To(g.Label("Only video files are allowed")),
			g.Dummy(0, 180),
		}...)
	} else {
		widgets = append(widgets, []g.Widget{
			g.Dummy(0, 15),
			g.Label("Convert List"),
			g.Row(
				g.Dummy(10, 0),
				g.Label(strings.Join(listOfVideos, "\n")).Wrapped(true),
			),
			g.Dummy(0, 10),
			g.Row(
				g.Label("size"),
				g.Dummy(10, 0),
				g.Combo("", sizeComboBoxLists[sizeComboBoxIdx], sizeComboBoxLists, &sizeComboBoxIdx).OnChange(func() {
					sizeToConvert = sizeComboBoxLists[sizeComboBoxIdx]
				}),
			),
			g.Row(
				g.Label("prefix"),
				g.Dummy(10, 0),
				g.InputText(&resultingFilePrefix),
			),
			g.Dummy(0, 30),
			g.Button("Execute").OnClick(onClickConvert).Disabled(isCurrentlyConverting),

			g.Label(convertingHelperMsg).Wrapped(true),
			g.Dummy(0, 10),
		}...)

		if isCurrentlyConverting {
			outputString := convertingFFmpegOutput.String()
			substring := ""

			// force ui update on doing conversion
			g.Update()

			if 100 > len(outputString) {
				substring = outputString
			} else {
				substring = outputString[len(outputString)-100:]
			}

			widgets = append(widgets, []g.Widget{
				g.Label(substring).Wrapped(true),
			}...)
		}
	}

	return widgets
}

func loop() {
	g.SingleWindow().Layout(
		myLayouts()...,
	)
}

func main() {
	go func() {
		ffmpegCheck()
		if !isFfmpegReady {
			updateEnvPath()
			ffmpegCheck()
			downloadFfmpeg()
			ffmpegCheck()
		}
	}()

	wnd := g.NewMasterWindow(fmt.Sprintf("video converter - %s", VERSION), 400, 400, g.MasterWindowFlagsNotResizable)

	g.Context.GetPlatform().SetDropCallback(func(filenames []string) {
		if isCurrentlyConverting {
			return
		}

		if 0 < len(listOfVideos) {
			listOfVideos = []string{}
		}

		if 0 < len(filenames) {
			for _, filename := range filenames {
				mimetype, err := detectFileMimetype(filename)
				if nil != err {
					panic(err)
				}

				if strings.HasPrefix(mimetype, "video/") {
					listOfVideos = append(listOfVideos, filename)
				}
			}
		}
	})

	wnd.Run(loop)
}
