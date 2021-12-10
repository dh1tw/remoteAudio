package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/hraban/opus.v2"
)

func checkAudioParameterValues() error {

	if chs := viper.GetInt("input-device.channels"); chs < 1 || chs > 2 {
		return &parmError{
			parm: "input-device.channels",
			msg:  "allowed values are [1 (Mono), 2 (Stereo)]",
		}
	}

	if chs := viper.GetInt("output-device.channels"); chs < 1 || chs > 2 {
		return &parmError{
			parm: "output-device.channels",
			msg:  "allowed values are [1 (Mono), 2 (Stereo)]",
		}
	}

	opusBw := viper.GetString("opus.max-bandwidth")
	if _, err := getOpusMaxBandwith(opusBw); err != nil {
		return &parmError{
			parm: "opus.max-bandwidth",
			msg:  "allowed values are NARROWBAND, MEDIUMBAND, WIDEBAND, SUPERWIDEBAND, FULLBAND",
		}
	}

	opusApp := viper.GetString("opus.application")
	if _, err := getOpusApplication(opusApp); err != nil {
		return &parmError{
			parm: "opus.application",
			msg:  "allowed values are VOIP, AUDIO or RESTRICTED_LOWDELAY",
		}
	}

	if viper.GetInt("opus.bitrate") < 6000 || viper.GetInt("opus.bitrate") > 510000 {
		return &parmError{
			parm: "opus.bitrate",
			msg:  "allowed values are [6000...510000]",
		}
	}

	if viper.GetInt("opus.complexity") < 0 || viper.GetInt("opus.complexity") > 10 {
		return &parmError{
			parm: "opus.complexity",
			msg:  "allowed values are [0...10]",
		}
	}

	opusFrameLength := float64(viper.GetInt("audio.frame-length")) / 48000
	if opusFrameLength != 0.0025 &&
		opusFrameLength != 0.005 &&
		opusFrameLength != 0.01 &&
		opusFrameLength != 0.02 &&
		opusFrameLength != 0.04 &&
		opusFrameLength != 0.06 {
		return &parmError{
			parm: "audio.frame-length",
			msg: `division of audio.frame-length/input-device.samplerate must
result in 2.5, 5, 10, 20, 40, 60ms for the opus codec`,
		}
	}

	if viper.GetInt("audio.rx-buffer-length") <= 0 {
		return &parmError{
			parm: "audio.rx-buffer-length",
			msg:  "value must be > 0",
		}
	}

	return nil
}

type parmError struct {
	parm string
	msg  string
}

func (p *parmError) Error() string {
	return fmt.Sprintf("%v: %v\n", p.parm, p.msg)
}

// getOpusApplication returns the integer representation of a
// Opus application value string (typically read from application settings)
func getOpusApplication(app string) (opus.Application, error) {
	switch strings.ToLower(app) {
	case "audio":
		return opus.AppAudio, nil
	case "restricted_lowdelay":
		return opus.AppRestrictedLowdelay, nil
	case "voip":
		return opus.AppVoIP, nil
	}
	return 0, errors.New("unknown opus application value")
}

// getOpusMaxBandwith returns the integer representation of an
// Opus max bandwidth value string (typically read from application settings)
func getOpusMaxBandwith(maxBw string) (opus.Bandwidth, error) {
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
