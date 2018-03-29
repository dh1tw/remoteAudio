package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dh1tw/remoteAudio/utils"
	"github.com/spf13/viper"
	"gopkg.in/hraban/opus.v2"
)

func checkAudioParameterValues() bool {

	ok := true

	if chs := viper.GetInt("input-device.channels"); chs < 1 || chs > 2 {
		fmt.Println(parmError{"input-device.channels", "allowed values are [1 (Mono), 2 (Stereo)]"})
		ok = false
	}

	if chs := viper.GetInt("output-device.channels"); chs < 1 || chs > 2 {
		fmt.Println(parmError{"output-device.channels", "allowed values are [1 (Mono), 2 (Stereo)]"})
		ok = false
	}

	if codec := strings.ToUpper(viper.GetString("audio.codec")); codec != "OPUS" && codec != "PCM" {
		fmt.Println(parmError{"audio.codec", "allowed values are [OPUS, PCM]"})
		ok = false
	}

	if strings.ToUpper(viper.GetString("audio.codec")) == "PCM" {

		if viper.GetFloat64("pcm.samplerate") < 0 {
			fmt.Println(parmError{"pcm.samplerate", "value must be > 0"})
			ok = false
		}

		if viper.GetFloat64("pcm.samplerate")/viper.GetFloat64("input-device.samplerate") < 1/256 ||
			viper.GetFloat64("pcm.samplerate")/viper.GetFloat64("input-device.samplerate") > 256 ||
			viper.GetFloat64("input-device.samplerate")/viper.GetFloat64("pcm.samplerate") < 1/256 ||
			viper.GetFloat64("input-device.samplerate")/viper.GetFloat64("pcm.samplerate") > 256 {
			fmt.Println(parmError{"pcm.samplerate", "ratio between input-device & pcm samplerate must be < 256"})
			ok = false
		}

		if viper.GetInt("pcm.bitdepth") != 8 &&
			viper.GetInt("pcm.bitdepth") != 12 &&
			viper.GetInt("pcm.bitdepth") != 16 &&
			viper.GetInt("pcm.bitdepth") != 24 {
			fmt.Println(parmError{"pcm.bitdepth", "allowed values are [8, 12, 16, 24]"})
			ok = false
		}

		if chs := viper.GetInt("pcm.channels"); chs >= 1 && chs <= 2 {
			fmt.Println(parmError{"pcm.channels", "allowed values are [1 (Mono), 2 (Stereo)]"})
			ok = false
		}

		if viper.GetInt("pcm.resampling-quality") < 0 || viper.GetInt("pcm.resampling-quality") > 4 {
			fmt.Println(parmError{"pcm.resampling-quality", "allowed values are [0...4]"})
			ok = false
		}

		if viper.GetInt("audio.frame-length") <= 0 {
			fmt.Println(parmError{"audio.frame-length", "value must be > 0"})
			ok = false
		}
	}

	if strings.ToUpper(viper.GetString("audio.codec")) == "OPUS" {
		opusApps := []string{"RESTRICTED_LOWDELAY", "VOIP", "AUDIO"}
		opusApp := strings.ToUpper(viper.GetString("opus.application"))
		if !utils.StringInSlice(opusApp, opusApps) {
			fmt.Println(parmError{"opus.application", "allowed values are VOIP, AUDIO or RESTRICTED_LOWDELAY"})
			ok = false
		}

		opusMaxBws := []string{"NARROWBAND", "MEDIUMBAND", "WIDEBAND", "SUPERWIDEBAND", "FULLBAND"}
		opusBw := strings.ToUpper(viper.GetString("opus.max-bandwidth"))
		if !utils.StringInSlice(opusBw, opusMaxBws) {
			fmt.Println(parmError{"opus.max-bandwidth", "allowed values are NARROWBAND, MEDIUMBAND, WIDEBAND, SUPERWIDEBAND, FULLBAND"})
			ok = false
		}

		if viper.GetInt("opus.bitrate") < 6000 || viper.GetInt("opus.bitrate") > 510000 {
			fmt.Println(parmError{"opus.bitrate", "allowed values are [6000...510000]"})
			ok = false
		}

		if viper.GetInt("opus.complexity") < 0 || viper.GetInt("opus.complexity") > 10 {
			fmt.Println(parmError{"opus.complexity", "allowed values are [0...10]"})
			ok = false
		}

		opusFrameLength := float64(viper.GetInt("audio.frame-length")) / viper.GetFloat64("input-device.samplerate")
		if opusFrameLength != 0.0025 &&
			opusFrameLength != 0.005 &&
			opusFrameLength != 0.01 &&
			opusFrameLength != 0.02 &&
			opusFrameLength != 0.04 &&
			opusFrameLength != 0.06 {
			fmt.Println(parmError{"audio.frame-length", "division of audio.frame-length/input-device.samplerate must result in 2.5, 5, 10, 20, 40, 60ms for the opus codec"})
			ok = false
		}
	}

	if viper.GetInt("audio.rx-buffer-length") <= 0 {
		fmt.Println(parmError{"audio.rx-buffer-length", "value must be > 0"})
		ok = false
	}

	return ok
}

type parmError struct {
	parm string
	msg  string
}

func (p *parmError) String() {
	fmt.Printf("%v: %v\n", p.parm, p.msg)
}

// GetOpusApplication returns the integer representation of a
// Opus application value string (typically read from application settings)
func GetOpusApplication(app string) (opus.Application, error) {
	switch app {
	case "audio":
		return opus.AppAudio, nil
	case "restricted_lowdelay":
		return opus.AppRestrictedLowdelay, nil
	case "voip":
		return opus.AppVoIP, nil
	}
	return 0, errors.New("unknown opus application value")
}

// GetOpusMaxBandwith returns the integer representation of an
// Opus max bandwitdh value string (typically read from application settings)
func GetOpusMaxBandwith(maxBw string) (opus.Bandwidth, error) {
	switch strings.ToLower(maxBw) {
	case "narrowband":
		return opus.Narrowband, nil
	case "mediumband":
		return opus.Mediumband, nil
	case "wideband":
		return opus.Wideband, nil
	case "superwideband":
		return opus.SuperWideband, nil
	case "fullband":
		return opus.Fullband, nil
	}

	return 0, errors.New("unknown opus max bandwidth value")
}
