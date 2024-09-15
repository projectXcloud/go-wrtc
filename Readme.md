# GO-WRTC

This project provides a WebRTC audio streaming server built with Go and FFmpeg, encapsulated within a Docker development environment. The server captures live audio input, encodes it as Opus, and streams it via RTP to WebRTC clients over WebSocket connections. It supports both production mode (live audio capture) and test mode (streaming from a pre-recorded MP3 file).

## Prerequisites

The only prerequisite for running this project is Docker. The development environment, including Go and FFmpeg, is fully contained within a Docker Dev Container.

## Project Structure

- **cmd/go-wrtc/main.go**: Main application code for the WebRTC server.
- **go-web-test/web.go**: Simple web server for hosting WebRTC client files.

## Makefile Commands

The provided Makefile includes commands for running, building, and testing the project within the Docker environment.

### Commands

- **run**: Starts the server in normal (production) mode, capturing live audio from the default audio source.

  ```bash
  make run
  ```

- **runt**: Starts the server in test mode, streaming a pre-recorded MP3 file (`file2.mp3`).

  ```bash
  make runt
  ```

- **build**: Compiles the Go application and outputs the binary to the `./build` directory.

  ```bash
  make build
  ```

- **runweb**: Runs a simple web server for hosting WebRTC client files, useful for testing the WebRTC connection.

  ```bash
  make runweb
  ```

## Running the Server

### Production Mode (Live Audio Capture)

To run the server in production mode, capturing live audio from your system’s default audio source:

```bash
make run
```

### Test Mode (Streaming Pre-recorded Audio)

To run the server in test mode, which streams a pre-recorded MP3 file (`file2.mp3`), use:

```bash
make runt
```

This mode is useful for testing the server’s functionality without needing live audio input.

## Testing the WebRTC Streaming

### Step 1: Run the Web Server

Start the web server to host the WebRTC client files:

```bash
make runweb
```

By default, this will start a web server on `http://localhost:8000`. Navigate to this URL in a WebRTC-compatible browser to connect as a client and test the audio stream.

### Step 2: Playing Audio with FFplay (Optional)

To validate the audio stream independently of the WebRTC client, you can use `ffplay` to play an MP3 file on loop:

```bash
ffplay -nodisp -loop 0 file2.mp3
```

This command will play the MP3 file in a loop without displaying video, which is useful for audio testing and quality checks.

## Configuration

### WebRTC Configuration

- The server uses STUN and TURN servers for NAT traversal. The TURN server is currently hardcoded:

  - TURN URL: `turn:freeturn.net:3478`
  - Username: `free`
  - Password: `free`

These configurations can be modified directly in the `newPeerConnection` function within `main.go`.

### FFmpeg Audio Encoding

The server uses FFmpeg to encode audio to Opus format. Key parameters include:

- Sample Rate: 48 kHz
- Channels: Stereo (2 channels)
- Bitrate: 128 kbps
- Opus Payload Type: 111

FFmpeg is included within the Docker environment, so no additional installation is required.
