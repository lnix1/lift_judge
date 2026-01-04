## WARNING
If you are reading this, for the love of god do not try to replicate what i've done here... I wanted to avoid the overhead, GIL and dependencies when packaging my application of using the python wrapper for MediaPipe, but generating a custom compilation of my script using bazel and the mediapipe repo was a nightmare (largely due to my learning it for the first time). Just save yourself the time and use my compiled version with the ```.task``` file here and do not waste your time replicating the compilation here...

## Compilation Instructions
1. Add the ```annotator.cpp`` file to the the ```mediapipe_repo``` in the folder ```mediapipe_repo/mediapipe/examples/annotator/```
2. Run the following command from the ```mediapipe_repo```:
- ```bazel build -c opt --define MEDIAPIPE_DISABLE_GPU=1 //mediapipe/examples/annotator:annotator_mp```
3. ```cp``` the ```annotator_mp``` file from ```bazel-bin/mediapipe/examples/annotator/``` directory into the project ```internal/annotators/media_pipe/``` directory
