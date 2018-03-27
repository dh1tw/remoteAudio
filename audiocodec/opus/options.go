package opus

import opus "gopkg.in/hraban/opus.v2"

type Option func(*Options)

type Options struct {
	Name        string
	Samplerate  int
	Channels    int
	Application opus.Application
}
