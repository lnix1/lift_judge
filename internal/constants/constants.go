package constants

const (
        NumSlots   = 10
        SlotSize   = 512 * 1024
        HeaderSize = 128
        TotalSize  = HeaderSize + (NumSlots * (HeaderSize + SlotSize))
        ShmPath    = "/dev/shm/camera_ring_buffer"

	// Assuming we use max frame rate on R-Pi Camera of 30fps & 640x640 w/ default resolution, this gives max of roughly 500MB of data
	MaxRecordingSeconds = 30 
	FramesPerSecond = 30
	MaxRecordedFrames = MaxRecordingSeconds * FramesPerSecond
	MaxRecordingSize = MaxRecordedFrames * (HeaderSize + SlotSize)
        ShmPathRecording    = "/dev/shm/camera_recording_buffer"
	
	StatusEmpty      = 0
	StatusRaw        = 1
	StatusProcessing = 2
	StatusReady      = 3
)
