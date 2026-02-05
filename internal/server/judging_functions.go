package server

import (
        "encoding/binary"
	"log"

	constants "github.com/lnix1/lift_judge/internal/constants"
)

func (apiCfg *ApiCfg) judgeSquat() bool {
	// The rule i came up with here was to judge that a lifter hit depth if the min observed diff between hip and knee  
	// dipped below 5% of the max diff (i.e. standing up straight at the start of the lift) for both left & right
	// This should adjust for distance from the camera.

	completedLift := false

	var minHipKneeDiffLeft int = 1000
	var maxHipKneeDiffLeft int = 0

	var minHipKneeDiffRight int = 1000
	var maxHipKneeDiffRight int = 0

	for i:=0; i < apiCfg.WriterCfg.RecordWriteIndex; i++ {
		blockStart := i * (constants.HeaderSize + constants.SlotSize)
		jointsStart := blockStart + 8 // double check this?
		
		status := apiCfg.WriterCfg.RecordedData[blockStart]
                
		if status == constants.StatusReady {
			leftKnee_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+2 : jointsStart+4])
			leftHip_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+10 : jointsStart+12])
			
			rightKnee_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+6 : jointsStart+8])
			rightHip_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+14 : jointsStart+16])

			leftDiff := int(leftKnee_Y) - int(leftHip_Y)
			if leftDiff > maxHipKneeDiffLeft {
				maxHipKneeDiffLeft = leftDiff
			}
			if leftDiff < minHipKneeDiffLeft {
				minHipKneeDiffLeft = leftDiff
			}
			log.Printf("left knee: %d, left hip: %d, diff: %d, min diff: %d, max diff: %d", leftKnee_Y, leftHip_Y, leftDiff, minHipKneeDiffLeft, maxHipKneeDiffLeft)
			
			rightDiff := int(rightKnee_Y) - int(rightHip_Y)
			if rightDiff > maxHipKneeDiffRight {
				maxHipKneeDiffRight = rightDiff
			}
			if rightDiff < minHipKneeDiffRight {
				minHipKneeDiffRight = rightDiff
			}
		}
        }
	if (float64(minHipKneeDiffLeft) < (float64(maxHipKneeDiffLeft) * 0.1)) && (float64(minHipKneeDiffRight) < (float64(maxHipKneeDiffRight) * 0.05)) {
		completedLift = true
	}
	return completedLift
}

func (apiCfg *ApiCfg) judgeDeadlift() bool {
	// The rule here is that until we record a frame where both wrists are below the knees, 
	// we are checking to record the maximum knee-hip pixel distance, and once we record wrists below knees
	// we switch to record a new max knee-hip diff. If the second max is within 5% of the first, we judge success.

	completedLift := false

	var maxHipKneeDiffLeft_1 uint16 = 0
	var maxHipKneeDiffLeft_2 uint16 = 0

	var maxHipKneeDiffRight_1 uint16 = 0
	var maxHipKneeDiffRight_2 uint16 = 0

	isPrePhase := true
	maxHipKneeDiffPtrLeft := &maxHipKneeDiffLeft_1
	maxHipKneeDiffPtrRight := &maxHipKneeDiffRight_1

	for i:=0; i < apiCfg.WriterCfg.RecordWriteIndex; i++ {
		blockStart := i * (constants.HeaderSize + constants.SlotSize)
		jointsStart := blockStart + 8
		
		status := apiCfg.WriterCfg.RecordedData[blockStart]
                
		if status == constants.StatusReady {
			leftKnee_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+2 : jointsStart+4])
			leftHip_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+10 : jointsStart+12])
			leftWrist_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+34 : jointsStart+36])
			
			rightKnee_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+6 : jointsStart+8])
			rightHip_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+14 : jointsStart+16])
			rightWrist_Y := binary.LittleEndian.Uint16(apiCfg.WriterCfg.RecordedData[jointsStart+38 : jointsStart+40])

			leftDiffWrist := int(leftKnee_Y) - int(leftWrist_Y)
			rightDiffWrist := int(rightKnee_Y) - int(rightWrist_Y)
			if isPrePhase == true && leftDiffWrist < 0 && rightDiffWrist < 0 {
				isPrePhase = false
				maxHipKneeDiffPtrLeft = &maxHipKneeDiffLeft_2
				maxHipKneeDiffPtrRight = &maxHipKneeDiffRight_2
			}

			leftDiffHip := leftKnee_Y - leftHip_Y 
			rightDiffHip := rightKnee_Y - rightHip_Y
			
			if leftDiffHip > *maxHipKneeDiffPtrLeft {
				*maxHipKneeDiffPtrLeft = leftDiffHip
			}
			if rightDiffHip > *maxHipKneeDiffPtrRight {
				*maxHipKneeDiffPtrRight = rightDiffHip
			}
		}
        }
	if (float64(maxHipKneeDiffLeft_2) > (float64(maxHipKneeDiffLeft_1) * 0.95)) && (float64(maxHipKneeDiffRight_2) > (float64(maxHipKneeDiffRight_1) * 0.95)) {
		completedLift = true
	}
	return completedLift
}
