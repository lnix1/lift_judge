package constants

const (
	DetectionConfidence = 0.8

        NumSlots   = 30
        SlotSize   = 512 * 1024
        HeaderSize = 128
        TotalSize  = HeaderSize + (NumSlots * (HeaderSize + SlotSize))
        ShmPath    = "/dev/shm/camera_ring_buffer"
        ShmPathCpp    = "/camera_ring_buffer"
	AnnotationTriggerSem = "/camera_frame_sem"

	MaxRecordingSeconds = 30 
	FramesPerSecond = 30
	MaxRecordedFrames = MaxRecordingSeconds * FramesPerSecond
	MaxRecordingSize = MaxRecordedFrames * (HeaderSize + SlotSize)
        ShmPathRecording = "/dev/shm/camera_recording_buffer"
        ShmPathRecordingCpp = "/camera_recording_buffer"
	
	StatusEmpty      = 0
	StatusRaw        = 1
	StatusProcessing = 2
	StatusReady      = 3

	PixelsHeight 	= 640
	PixelsWidth	= 640
)
