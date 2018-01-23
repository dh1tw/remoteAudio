package audio

// AdjustChannels is a helper function which will either add or
// remove a channel (e.g. converting from Mono to Stereo or vice versa).
func AdjustChannels(iChs, oChs int, audioFrames []float32) []float32 {
	// mono -> stereo
	if iChs == 1 && oChs == 2 {
		res := make([]float32, 0, len(audioFrames)*2)
		// left channel = right channel
		for _, frame := range audioFrames {
			res = append(res, frame)
			res = append(res, frame)
		}
		return res
	}

	// stereo -> mono
	res := make([]float32, 0, len(audioFrames)/2)
	// chop off the right channel
	for i := 0; i < len(audioFrames); i += 2 {
		res = append(res, audioFrames[i])
	}
	return res
}

// AdjustVolume adjusts the volume in all the audio frames within
// an audio buffer
func AdjustVolume(volume float32, aBuffer []float32) {
	for i := 0; i < len(aBuffer); i++ {
		aBuffer[i] *= volume
	}
}
