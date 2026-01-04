g++ -O3 -o annotator annotator.cpp \
    -I/usr/local/include/opencv4 \
    -L/usr/local/lib \
    -Wl,-rpath,/usr/local/lib \
    -lopencv_core -lopencv_imgproc -lopencv_dnn -lopencv_imgcodecs -lrt -lpthread
