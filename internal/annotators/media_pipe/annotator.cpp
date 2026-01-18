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

// Macros for parse arguments
#include "absl/flags/flag.h"
#include "absl/flags/parse.h"

// Namespaces for clarity
using namespace mediapipe::tasks::vision::pose_landmarker;
using namespace mediapipe::tasks::core;
using namespace mediapipe::tasks::vision::core; // For RunningMode

ABSL_FLAG(int, headersize, 0, "Size of the shared memory header");
ABSL_FLAG(int, slotsize, 0, "Size of the slots allocaated in shared memory for a single frame");
ABSL_FLAG(int, numslots, 0, "Size of the slots allocaated in shared memory for a single frame");
ABSL_FLAG(float, detectionconfidence, 0, "Min confidence required to plot a joint detection");
ABSL_FLAG(bool, isring, false, "Indicates whether this is a continuous process or annotation of a single recorded video");
ABSL_FLAG(std::string, shmpath, "", "Path to the shared memory block");

// MediaPipe Landmark Indices (0-32) mapped to your SHM order
// order: L_Knee, R_Knee, L_Hip, R_Hip, L_Shoulder, R_Shoulder, L_Elbow, R_Elbow, L_Wrist, R_Wrist
const int MP_MAP[10] = {25, 26, 23, 24, 11, 12, 13, 14, 15, 16};

int main(int argc, char* argv[]) {
    absl::ParseCommandLine(argc, argv);

    int HEADER_SIZE = absl::GetFlag(FLAGS_headersize);
    int SLOT_SIZE = absl::GetFlag(FLAGS_slotsize);
    int NUM_SLOTS = absl::GetFlag(FLAGS_numslots);
    float DETECTION_CONFIDENCE = absl::GetFlag(FLAGS_detectionconfidence);
    bool IS_RING = absl::GetFlag(FLAGS_isring);
    std::string shm_path = absl::GetFlag(FLAGS_shmpath);
    int TOTAL_SIZE = HEADER_SIZE * IS_RING + (NUM_SLOTS * (HEADER_SIZE + SLOT_SIZE));

    if (argc < 7) {
        std::cerr << "Missing configuration arguments" << std::endl;
        return 1;
    }

    // 1. Map the Shared Memory
    int fd = shm_open(shm_path.c_str(), O_RDWR, 0666);
    if (fd < 0) { std::cerr << "SHM Open failed\n"; return 1; }
    uint8_t* ptr = (uint8_t*)mmap(NULL, TOTAL_SIZE, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);

    // 2. Initialize MediaPipe Pose Landmarker
    auto options = std::make_unique<PoseLandmarkerOptions>();
    options->base_options.model_asset_path = "internal/annotators/media_pipe/pose_landmarker_lite.task";
    
    // Explicitly using the full namespace to resolve the "not declared" error
    options->running_mode = mediapipe::tasks::vision::core::RunningMode::VIDEO;
    options->min_pose_detection_confidence = DETECTION_CONFIDENCE;

    auto landmarker_res = PoseLandmarker::Create(std::move(options));
    if (!landmarker_res.ok()) {
        std::cerr << "MediaPipe Init Failed: " << landmarker_res.status().message() << "\n";
        return 1;
    }
    auto landmarker = std::move(landmarker_res.value());

    std::cout << "MediaPipe Annotator online (VIDEO mode)." << std::endl;

    uint64_t timestamp_ms = 0;

    if (IS_RING == true) {
    	while (true) {
            uint8_t write_index = *ptr;

            uint8_t* block_ptr = ptr + (HEADER_SIZE * IS_RING) + (write_index * (HEADER_SIZE + SLOT_SIZE));
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
    } else {
	for (int i = 0; i < NUM_SLOTS; i++) {
            uint8_t* block_ptr = ptr + (HEADER_SIZE * IS_RING) + (i * (HEADER_SIZE + SLOT_SIZE));
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
    }
}
