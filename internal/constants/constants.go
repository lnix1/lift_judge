package constants

const (
        NumSlots   = 10
        SlotSize   = 512 * 1024
        HeaderSize = 128
        TotalSize  = HeaderSize + (NumSlots * (HeaderSize + SlotSize))
        ShmPath    = "/dev/shm/camera_ring_buffer"
	
	StatusEmpty      = 0
	StatusRaw        = 1
	StatusProcessing = 2
	StatusReady      = 3
)
