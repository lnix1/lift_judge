# LiftJudge

A computer vision app for judging whether powerlifters meet depth on squats or lock out on deadlift.

![Demo gif](LiftJudgeGif.gif)


## Motivation 

I've spent plenty of time stressing over depth or lockout with teammates when peaking before a powerlifting meet. I wanted a tool to give clear annotation of joint positions & tracking, as well as pixel-level evaluation of a lifter's depth / lockout. 

I also wanted to challenge myself to get this running on an edge device, specifically a Raspberry Pi 5. You could run a larger model quite easily with a Hailo module, but I liked the challenge of identifying and optimizing the implementation of a pose estimation model such that we could run this on the PI's CPU alone, and also run a server for users to use the tool in their browser.



## Hardware, App Architecture, User Flow and Benchmarks

**Hardware:**

- Raspberry Pi 5
- Dual M.2 Pi HAT
- Pi cooling fan
- Pi AI camera (though I opted not to use the AI deployment functionality of the camera and just used it as a normal camera)
- M.2 SSD 

The core functionality of the app feeds to frames from the PI camera to a ring buffer, then prods a custom annotator that leverages the MediaPipe Pose Landmarker Lite model to annotate 10 joints. A golang server then trails and serves up the frames. This is optimized where about half the frames are annotated live and this process uses aroun 30% of the Pi's 4 CPU cores.

A user can see a live view to see where their joints are position at lift start. They can then start and stop a recording, at which point any frames that are not annotated live are annotated and a judgment is rendered. The user can then choose to save the lift. Users can also view old lifts and delete lifts. This functionality is offered for recording / judging for both Squat and Deadlift, with custom judging functions for each that use the specific join estimation values.

This is built in such a way that only one user can record at a time (naturally since there is only one camera) but each video is tagged to the recording user's account.

The project has it's own handrolled authentication (using JWT's) and postgress database for storing user login information & saved video links. 



## Quick Start

The whole project is packaged up using Docker, so getting the app up and running on a new Pi is quick and easy.

1. Ensure the Pi is at least a 5 and has the camera and SSD.
2. Ensure you have docker and are logged in via the CLI
3. Runn the following:

```
git clone https://github.com/lnix1/lift_judge.git
cd lift_judge
```

4. Create a ```.env``` file in the project root and fill with the following, specifying the username, password, & secreate string you would like (don't forget to adjust the ```DB_URL``` to use the same username and password as well):

```
# Application Settings
PLATFORM=prod
SECRET=change_this_to_random_string

# Database Credentials
DB_USER=postgres
DB_PASS=secure_password
DB_NAME=liftjudge

# Docker Network Connection (Keep host as 'db')
DB_URL=postgres://postgres:secure_password@db:5432/liftjudge?sslmode=disable
```

5. Run ```docker compose up -d```



## Usage

With the app running, open the tool by using a any browser on the same wifi as the Pi to open the URL below:

```raspberrypi.local:8080/```

Once open, click to create a new account. This requires just an email and password (which are stored in a postgres db also running locally via Docker with the password hashed).

Once created, use the email and password to login. You will be taken to a "video feed" page.

Position the camera about knee or hip height, directly in front of the lift about 10 feet away for optimal performance.

Choose the lift in the dropdown, then press the "play" button in the bottom left of the video feed to start recording. Hit the pause button that appears to stop the recording.
- At this point, the camera will slow for a moment while the recording is further processed and judged. After a few seconds, you will be taken to a page to view your recording with a lift result rendered.

If you want to save the lift, click save. You will then be taken back to the live feed.

You can retrieve and view any saved videos by expanding the burger icon in the top right. Each lift is tagged with the lift type, date and time of recording.


## Contributing

The app currently uses fairly naive judging formulas and only checks for depth / lockout (i.e. missing double bounces and other violations that would yield redlights for a lifter). I also haven't tackled benchpress yet. If you would like to contribute, please feel free to do so by forking and opening pull requests.
