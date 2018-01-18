package audio

import (
	"fmt"
	"io"
	"os"

	wav "github.com/youpy/go-wav"
)

func WavFile(path string) ([]AudioMsg, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	reader := wav.NewReader(file)

	format, err := reader.Format()
	if err != nil {
		return nil, err
	}

	fmt.Println(format)

	samplerate := float64(format.SampleRate)
	channels := int(format.NumChannels)
	frames := 4096

	msgs := []AudioMsg{}

	for {
		frame, err := reader.ReadSamples(uint32(frames))
		if err == io.EOF {
			break
		}

		data := make([]float32, 0, len(frame)*channels)

		for _, sample := range frame {
			data = append(data, float32(reader.FloatValue(sample, 0)))
			// if stereo add interleaved sample of right channel
			if channels == 2 {
				data = append(data, float32(reader.FloatValue(sample, 1)))
			}
		}

		fmt.Println(len(frame))

		msg := AudioMsg{
			Channels:   channels,
			Data:       data,
			Frames:     len(frame),
			Samplerate: samplerate,
		}
		msgs = append(msgs, msg)
	}

	msgs[len(msgs)-1].EOF = true

	return msgs, nil
}
