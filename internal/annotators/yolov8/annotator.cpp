#include <opencv2/opencv.hpp>
#include <opencv2/dnn.hpp>
#include <sys/mman.h>
#include <fcntl.h>
#include <unistd.h>
#include <iostream>

const int NUM_SLOTS = 10;
const int SLOT_SIZE = 512 * 1024;
const int HEADER_SIZE = 128;
const int TOTAL_SIZE = HEADER_SIZE + (NUM_SLOTS * (HEADER_SIZE + SLOT_SIZE));

// COCO Keypoint Indices for YOLOv8-Pose
enum JointIdx {
    L_Shoulder = 5, R_Shoulder = 6,
    L_Elbow = 7,    R_Elbow = 8,
    L_Wrist = 9,    R_Wrist = 10,
    L_Hip = 11,     R_Hip = 12,
    L_Knee = 13,    R_Knee = 14
};

int main() {
    // 1. Map the Shared Memory
    int fd = shm_open("/camera_ring_buffer", O_RDWR, 0666);
    if (fd < 0) { std::cerr << "SHM Open failed\n"; return 1; }
    
    uint8_t* ptr = (uint8_t*)mmap(NULL, TOTAL_SIZE, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);

    // 2. Load the Model
    // Place your yolov8n-pose.onnx in a 'models' folder
    cv::dnn::Net net = cv::dnn::readNetFromONNX("./internal/annotators/yolov8/yolov8n-pose.onnx");
    net.setPreferableBackend(cv::dnn::DNN_BACKEND_OPENCV);
    net.setPreferableTarget(cv::dnn::DNN_TARGET_CPU); // Change to DNN_TARGET_TIMVX for Pi AI Kit

    std::cout << "Annotator started. Waiting for frames..." << std::endl;

    while (true) {
        for (int i = 0; i < NUM_SLOTS; i++) {
            // Calculate slot block start
            uint8_t* block_ptr = ptr + HEADER_SIZE + (i * (HEADER_SIZE + SLOT_SIZE));
            uint8_t* status = block_ptr;

            if (*status == 1) { // StatusRaw
                *status = 2; // StatusProcessing

                uint32_t img_len = *(uint32_t*)(block_ptr + 4);
                uint8_t* jpeg_data = block_ptr + HEADER_SIZE;

                // Decode JPEG from RAM
                std::vector<uchar> input_vec(jpeg_data, jpeg_data + img_len);
                cv::Mat frame = cv::imdecode(input_vec, cv::IMREAD_COLOR);
                if (frame.empty()) { *status = 0; continue; }

                // 3. Inference: YOLOv8-Pose
                cv::Mat blob = cv::dnn::blobFromImage(frame, 1/255.0, cv::Size(640, 640), cv::Scalar(0,0,0), true, false);
                net.setInput(blob);
                cv::Mat output = net.forward(); // Shape: [1, 56, 8400]

                // Reshape to access rows (8400 candidates)
                cv::Mat rows = output.reshape(1, output.size[1]); 
                cv::transpose(rows, rows); // Now [8400, 56]

                float max_score = 0;
                int best_idx = -1;

                // Find the detection with the highest confidence
                for (int r = 0; r < rows.rows; r++) {
                    float score = rows.at<float>(r, 4);
                    if (score > max_score) {
                        max_score = score;
                        best_idx = r;
                    }
                }

                if (best_idx != -1 && max_score > 0.5) {
                    float* data = rows.ptr<float>(best_idx);
                    uint16_t* shm_joints = (uint16_t*)(block_ptr + 8);

                    // Mapping target joints to our SHM Header
                    int target_joints[] = {L_Knee, R_Knee, L_Hip, R_Hip, L_Shoulder, R_Shoulder, L_Elbow, R_Elbow, L_Wrist, R_Wrist};
                    
                    for (int j = 0; j < 10; j++) {
                        int coco_idx = target_joints[j];
                        // YOLOv8 Pose: keypoints start at index 5. 
                        // Each keypoint is (x, y, confidence)
                        float x = data[5 + coco_idx * 3];
                        float y = data[5 + coco_idx * 3 + 1];

                        // Scale coordinates back to frame size (assuming 640x640)
                        shm_joints[j * 2] = (uint16_t)x;     // X
                        shm_joints[j * 2 + 1] = (uint16_t)y; // Y

                        // Draw dots for the browser stream
                        cv::circle(frame, cv::Point(x, y), 6, cv::Scalar(0, 255, 0), -1);
                    }
                }

                // 4. Re-encode and Write back to RAM
                std::vector<uchar> out_buf;
                cv::imencode(".jpg", frame, out_buf);
                memcpy(jpeg_data, out_buf.data(), out_buf.size());
                *(uint32_t*)(block_ptr + 4) = (uint32_t)out_buf.size();

                *status = 3; // StatusReady
            }
        }
        usleep(1000); // 1ms backoff
    }
}
