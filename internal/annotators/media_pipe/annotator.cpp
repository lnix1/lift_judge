#include <opencv2/opencv.hpp>
#include <sys/mman.h>
#include <fcntl.h>
#include <unistd.h>
#include <iostream>
#include <vector>

// MediaPipe Task Headers
#include "mediapipe/tasks/cc/vision/pose_landmarker/pose_landmarker.h"
#include "mediapipe/tasks/cc/core/base_options.h"
#include "mediapipe/framework/formats/image_frame.h"
#include "mediapipe/framework/formats/image_frame_opencv.h"

// Namespaces for clarity
using namespace mediapipe::tasks::vision::pose_landmarker;
using namespace mediapipe::tasks::core;
using namespace mediapipe::tasks::vision::core; // For RunningMode

const int NUM_SLOTS = 10;
const int SLOT_SIZE = 512 * 1024;
const int HEADER_SIZE = 128;
const int TOTAL_SIZE = HEADER_SIZE + (NUM_SLOTS * (HEADER_SIZE + SLOT_SIZE));

// MediaPipe Landmark Indices (0-32) mapped to your SHM order
// order: L_Knee, R_Knee, L_Hip, R_Hip, L_Shoulder, R_Shoulder, L_Elbow, R_Elbow, L_Wrist, R_Wrist
const int MP_MAP[10] = {25, 26, 23, 24, 11, 12, 13, 14, 15, 16};

int main() {
    // 1. Map the Shared Memory
    int fd = shm_open("/camera_ring_buffer", O_RDWR, 0666);
    if (fd < 0) { std::cerr << "SHM Open failed\n"; return 1; }
    uint8_t* ptr = (uint8_t*)mmap(NULL, TOTAL_SIZE, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);

    // 2. Initialize MediaPipe Pose Landmarker
    auto options = std::make_unique<PoseLandmarkerOptions>();
    options->base_options.model_asset_path = "internal/annotators/media_pipe/pose_landmarker_lite.task";
    
    // Explicitly using the full namespace to resolve the "not declared" error
    options->running_mode = mediapipe::tasks::vision::core::RunningMode::VIDEO;
    options->min_pose_detection_confidence = 0.5f;

    auto landmarker_res = PoseLandmarker::Create(std::move(options));
    if (!landmarker_res.ok()) {
        std::cerr << "MediaPipe Init Failed: " << landmarker_res.status().message() << "\n";
        return 1;
    }
    auto landmarker = std::move(landmarker_res.value());

    std::cout << "MediaPipe Annotator online (VIDEO mode)." << std::endl;

    uint64_t timestamp_ms = 0;

    while (true) {
        for (int i = 0; i < NUM_SLOTS; i++) {
            uint8_t* block_ptr = ptr + HEADER_SIZE + (i * (HEADER_SIZE + SLOT_SIZE));
            uint8_t* status = block_ptr;

            if (*status == 1) { // StatusRaw
                *status = 2; // StatusProcessing

                uint32_t img_len = *(uint32_t*)(block_ptr + 4);
                uint8_t* jpeg_data = block_ptr + HEADER_SIZE;

                std::vector<uchar> input_vec(jpeg_data, jpeg_data + img_len);
                cv::Mat frame = cv::imdecode(input_vec, cv::IMREAD_COLOR);
                if (frame.empty()) { *status = 0; continue; }

                // 3. Manual Conversion (Replacing the missing MakeCpuImageFromMat)
                timestamp_ms += 33; 
                auto image_frame = std::make_unique<mediapipe::ImageFrame>(
                    mediapipe::ImageFormat::SRGB, frame.cols, frame.rows,
                    mediapipe::ImageFrame::kDefaultAlignmentBoundary);
                
		frame.copyTo(mediapipe::formats::MatView(image_frame.get()));

                mediapipe::Image mp_image(std::move(image_frame));

                // 4. Run Inference
		auto result_res = landmarker->DetectForVideo(mp_image, (int64_t)timestamp_ms, std::nullopt);
                if (result_res.ok()) {
                    auto result = std::move(result_res.value());
                    if (!result.pose_landmarks.empty()) {
                        auto& landmarks = result.pose_landmarks[0];
                        uint16_t* shm_joints = (uint16_t*)(block_ptr + 8);

                        for (int j = 0; j < 10; j++) {
			    auto& lm = landmarks.landmarks[MP_MAP[j]];
                            float x = lm.x * frame.cols;
                            float y = lm.y * frame.rows;

                            shm_joints[j * 2] = (uint16_t)x;
                            shm_joints[j * 2 + 1] = (uint16_t)y;

                            cv::circle(frame, cv::Point(x, y), 6, cv::Scalar(0, 255, 0), -1);
                        }
                    }
                }

                // 5. Re-encode
                std::vector<uchar> out_buf;
                cv::imencode(".jpg", frame, out_buf);
                memcpy(jpeg_data, out_buf.data(), out_buf.size());
                *(uint32_t*)(block_ptr + 4) = (uint32_t)out_buf.size();

                *status = 3; // StatusReady
            }
        }
        usleep(1000); 
    }
}
