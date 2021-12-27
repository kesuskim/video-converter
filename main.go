package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
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

	"golang.org/x/text/unicode/norm"

	g "github.com/AllenDang/giu"

	ffmpeg "github.com/u2takey/ffmpeg-go"

	_ "embed"
)

var IS_DEV = false

var listOfVideos []string
var isFfmpegReady bool
var tmpbinPath string
var isConversionPreparing bool
var isCurrentlyConverting bool
var convertingHelperMsg string
var convertingFFmpegOutput bytes.Buffer

var ffmpegCancelChannel = make(chan struct{})

var resComboBoxLists = []string{
	"original",
	"480p",
	"720p",
	"1080p",
}
var resComboBoxIdx int32 = 0
var resToUse = "original"

var audioCodecComboBoxLists = []string{
	"original",
	"AAC",
	"OPUS",
	"VORBIS",
}
var audioCodecComboBoxIdx int32 = 0
var audioCodecToUse = "original"

var videoCodecComboBoxLists = []string{
	"original",
	"H.264",
	"H.265",
}
var videoCodecComboBoxIdx int32 = 0
var videoCodecToUse = "original"

var containerFormatComboBoxLists = []string{
	"original",
	"mp4",
	"mkv",
}
var containerFormatComboBoxIdx int32 = 0
var containerFormatToUse = "original"

var resultingFilePrefix = "cvt-"

//go:embed res/NanumGothic-Regular.ttf
var fontBytes []byte

var (
	VERSION = "development"
)

func init() {
	g.SetDefaultFontFromBytes(fontBytes, 16)
}

func ffmpegOutputKwargs() ffmpeg.KwArgs {
	args := ffmpeg.KwArgs{
		"c:a": "copy",
		"c:v": "copy",
	}

	switch audioCodecToUse {
	case "AAC":
		args["c:a"] = "aac"
	case "OPUS":
		args["c:a"] = "libopus"
		args["b:a"] = "96k"
	case "VORBIS":
		args["c:a"] = "libvorbis"
	}

	switch videoCodecToUse {
	case "H.264":
		args["c:v"] = "libx264"
	case "H.265":
		args["c:v"] = "libx265"
	}

	switch resToUse {
	case "480p":
		args["filter:v"] = "scale=trunc(oh*a/2)*2:480"
	case "720p":
		args["filter:v"] = "scale=trunc(oh*a/2)*2:720"
	case "1080p":
		args["filter:v"] = "scale=trunc(oh*a/2)*2:1080"
	}

	return args
}

func checkFfmpegAndFfprobe() {
	isFfmpegReady = false

	if _, err := exec.LookPath("ffmpeg"); nil == err {
		if _, err := exec.LookPath("ffprobe"); nil == err {
			isFfmpegReady = true
		}
	}
}

func updateEnvPath() {
	tmpbinPath = path.Join(os.TempDir(), "bin")
	os.MkdirAll(tmpbinPath, os.FileMode(0755))

	os.Setenv("PATH", fmt.Sprintf("%s%c%s", os.Getenv("PATH"), os.PathListSeparator, tmpbinPath))
}

func downloadFileFromURL(url, filename string) error {
	resp, err := http.Get(url)
	if nil != err {
		return err
	}
	defer resp.Body.Close()

	if "windows" == runtime.GOOS {
		filename += ".exe"
	}

	f, err := os.OpenFile(path.Join(tmpbinPath, filename), os.O_CREATE|os.O_RDWR, os.FileMode(0755))
	if nil != err {
		return err
	}

	if _, err := io.Copy(f, resp.Body); nil != err {
		return err
	}

	return nil
}

func downloadGzippedFileFromURL(url, filename string) error {
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

	if "windows" == runtime.GOOS {
		filename += ".exe"
	}

	f, err := os.OpenFile(path.Join(tmpbinPath, filename), os.O_CREATE|os.O_RDWR, os.FileMode(0755))
	if nil != err {
		return err
	}

	if _, err := io.Copy(f, gzReader); nil != err {
		return err
	}

	return nil
}

func downloadFfmpegAndFfprobe() {
	switch runtime.GOOS {
	case "darwin":
		if "arm64" == runtime.GOARCH {
			downloadGzippedFileFromURL("https://github.com/eugeneware/ffmpeg-static/releases/download/b4.4/darwin-arm64.gz", "ffmpeg")
			downloadFileFromURL("https://github.com/descriptinc/ffmpeg-ffprobe-static/releases/download/b4.4.0-rc.11/ffprobe-darwin-arm64", "ffprobe")
		} else {
			downloadGzippedFileFromURL("https://github.com/eugeneware/ffmpeg-static/releases/download/b4.4/darwin-x64.gz", "ffmpeg")
			downloadGzippedFileFromURL("https://github.com/descriptinc/ffmpeg-ffprobe-static/releases/download/b4.4.0-rc.11/ffprobe-darwin-x64.gz", "ffprobe")
		}
	case "windows":
		downloadGzippedFileFromURL("https://github.com/eugeneware/ffmpeg-static/releases/download/b4.4/win32-x64.gz", "ffmpeg")
		downloadGzippedFileFromURL("https://github.com/descriptinc/ffmpeg-ffprobe-static/releases/download/b4.4.0-rc.11/ffprobe-win32-x64.gz", "ffprobe")

	default:
		panic("not supported OS")
	}
}

type ffprobeOutput struct {
	Streams []struct {
		Index              int    `json:"index"`
		CodecName          string `json:"codec_name"`
		CodecLongName      string `json:"codec_long_name"`
		Profile            string `json:"profile"`
		CodecType          string `json:"codec_type"`
		CodecTagString     string `json:"codec_tag_string"`
		CodecTag           string `json:"codec_tag"`
		Width              int    `json:"width,omitempty"`
		Height             int    `json:"height,omitempty"`
		CodedWidth         int    `json:"coded_width,omitempty"`
		CodedHeight        int    `json:"coded_height,omitempty"`
		ClosedCaptions     int    `json:"closed_captions,omitempty"`
		HasBFrames         int    `json:"has_b_frames,omitempty"`
		SampleAspectRatio  string `json:"sample_aspect_ratio,omitempty"`
		DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"`
		PixFmt             string `json:"pix_fmt,omitempty"`
		Level              int    `json:"level,omitempty"`
		ColorRange         string `json:"color_range,omitempty"`
		ColorSpace         string `json:"color_space,omitempty"`
		ColorTransfer      string `json:"color_transfer,omitempty"`
		ColorPrimaries     string `json:"color_primaries,omitempty"`
		ChromaLocation     string `json:"chroma_location,omitempty"`
		Refs               int    `json:"refs,omitempty"`
		IsAvc              string `json:"is_avc,omitempty"`
		NalLengthSize      string `json:"nal_length_size,omitempty"`
		RFrameRate         string `json:"r_frame_rate"`
		AvgFrameRate       string `json:"avg_frame_rate"`
		TimeBase           string `json:"time_base"`
		StartPts           int    `json:"start_pts"`
		StartTime          string `json:"start_time"`
		DurationTs         int    `json:"duration_ts"`
		Duration           string `json:"duration"`
		BitRate            string `json:"bit_rate"`
		BitsPerRawSample   string `json:"bits_per_raw_sample,omitempty"`
		NbFrames           string `json:"nb_frames"`
		Disposition        struct {
			Default         int `json:"default"`
			Dub             int `json:"dub"`
			Original        int `json:"original"`
			Comment         int `json:"comment"`
			Lyrics          int `json:"lyrics"`
			Karaoke         int `json:"karaoke"`
			Forced          int `json:"forced"`
			HearingImpaired int `json:"hearing_impaired"`
			VisualImpaired  int `json:"visual_impaired"`
			CleanEffects    int `json:"clean_effects"`
			AttachedPic     int `json:"attached_pic"`
			TimedThumbnails int `json:"timed_thumbnails"`
		} `json:"disposition"`
		Tags struct {
			Language    string `json:"language"`
			HandlerName string `json:"handler_name"`
			VendorID    string `json:"vendor_id"`
		} `json:"tags"`
		SampleFmt     string `json:"sample_fmt,omitempty"`
		SampleRate    string `json:"sample_rate,omitempty"`
		Channels      int    `json:"channels,omitempty"`
		ChannelLayout string `json:"channel_layout,omitempty"`
		BitsPerSample int    `json:"bits_per_sample,omitempty"`
	} `json:"streams"`
	Format struct {
		Filename       string `json:"filename"`
		NbStreams      int    `json:"nb_streams"`
		NbPrograms     int    `json:"nb_programs"`
		FormatName     string `json:"format_name"`
		FormatLongName string `json:"format_long_name"`
		StartTime      string `json:"start_time"`
		Duration       string `json:"duration"`
		Size           string `json:"size"`
		BitRate        string `json:"bit_rate"`
		ProbeScore     int    `json:"probe_score"`
		Tags           struct {
			MajorBrand       string `json:"major_brand"`
			MinorVersion     string `json:"minor_version"`
			CompatibleBrands string `json:"compatible_brands"`
			Encoder          string `json:"encoder"`
		} `json:"tags"`
	} `json:"format"`
}

func detectWithFfprobe(filename string) (ffprobeOutput, error) {
	var ret ffprobeOutput
	probeOutputString, err := ffmpeg.Probe(filename)
	if nil != err {
		return ret, err
	}

	err = json.Unmarshal([]byte(probeOutputString), &ret)
	if nil != err {
		return ret, err
	}

	return ret, nil
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

func onClickCancel() {
	ffmpegCancelChannel <- struct{}{}
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
			fileext := filepath.Ext(videoPath)
			filenameWithoutExt := strings.TrimRight(filename, fileext)

			fileprefix := resultingFilePrefix
			if "original" != resToUse {
				fileprefix += fmt.Sprintf("%s-", resToUse)
			}

			switch containerFormatToUse {
			case "mp4":
				fileext = ".mp4"
			case "mkv":
				fileext = ".mkv"
			}

			convertedPath := fmt.Sprintf("%s/%s%s%s", dirname, fileprefix, filenameWithoutExt, fileext)
			convertingHelperMsg = fmt.Sprintf("currently converting:\n %s\ndestination:\n %s", videoPath, convertedPath)

			isCurrentlyConverting = true
			ffmpegCmd := ffmpeg.Input(videoPath).Output(convertedPath, ffmpegOutputKwargs()).OverWriteOutput().WithErrorOutput(&convertingFFmpegOutput).Compile()
			ffmpegFinishChannel := make(chan struct{})

			go func() {
				select {
				case <-ffmpegCancelChannel:
					isConversionPreparing = false
					isCurrentlyConverting = false
					ffmpegCmd.Process.Kill()
				case <-ffmpegFinishChannel:
					break
				}
			}()

			if err := ffmpegCmd.Run(); nil != err {
				convertingHelperMsg = err.Error()
			} else {
				convertingHelperMsg = fmt.Sprintf("finish conversion\ndestination:\n %s", convertedPath)
			}

			close(ffmpegFinishChannel)
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
				g.Label("resolution"),
				g.Dummy(10, 0),
				g.Combo("", resComboBoxLists[resComboBoxIdx], resComboBoxLists, &resComboBoxIdx).OnChange(func() {
					resToUse = resComboBoxLists[resComboBoxIdx]
				}),
			),
			g.Row(
				g.Label("prefix"),
				g.Dummy(10, 0),
				g.InputText(&resultingFilePrefix),
			),
			g.Row(
				g.Label("audio codec"),
				g.Dummy(10, 0),
				g.Combo("", audioCodecComboBoxLists[audioCodecComboBoxIdx], audioCodecComboBoxLists, &audioCodecComboBoxIdx).OnChange(func() {
					audioCodecToUse = audioCodecComboBoxLists[audioCodecComboBoxIdx]
				}),
			),
			g.Row(
				g.Label("video codec"),
				g.Dummy(10, 0),
				g.Combo("", videoCodecComboBoxLists[videoCodecComboBoxIdx], videoCodecComboBoxLists, &videoCodecComboBoxIdx).OnChange(func() {
					videoCodecToUse = videoCodecComboBoxLists[videoCodecComboBoxIdx]
				}),
			),
			g.Row(
				g.Label("container"),
				g.Dummy(10, 0),
				g.Combo("", containerFormatComboBoxLists[containerFormatComboBoxIdx], containerFormatComboBoxLists, &containerFormatComboBoxIdx).OnChange(func() {
					containerFormatToUse = containerFormatComboBoxLists[containerFormatComboBoxIdx]
				}),
			),
			g.Dummy(0, 10),
			g.Row(
				g.Button("Execute").OnClick(onClickConvert).Disabled(isCurrentlyConverting),
				g.Button("Cancel").OnClick(onClickCancel).Disabled(!isCurrentlyConverting),
			),

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
		checkFfmpegAndFfprobe()
		if !isFfmpegReady {
			updateEnvPath()
			checkFfmpegAndFfprobe()
			downloadFfmpegAndFfprobe()
			checkFfmpegAndFfprobe()
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
				filestat, err := os.Stat(filename)
				if nil != err || filestat.IsDir() {
					continue
				}

				filename = norm.NFC.String(filename)

				// TODO
				// if ffprobeOutput, err := detectWithFfprobe(filename); nil == err {
				if _, err := detectWithFfprobe(filename); nil == err {
					listOfVideos = append(listOfVideos, filename)
					// for _, ffprobeOutputStream := range ffprobeOutput.Streams {
					// 	codecName := ffprobeOutputStream.CodecName
					// }
				}
			}
		}
	})

	wnd.Run(loop)
}
