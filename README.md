# AI Subtitle Synchronizer

A completely offline, microservice-based system that uses OpenAI Whisper and Dynamic Time Warping (DTW) to perfectly synchronize out-of-sync subtitles to video audio — **even if the subtitles and the audio are in completely different languages.**

## Features
- **100% Offline Capable**: Uses local Whisper models and Argos neural machine translation. No API keys required.
- **Cross-Language Syncing**: Automatically translates subtitles internally to match the spoken language so DTW alignment works perfectly. Returns the original language.
- **Robust Alignment**: Uses a full Sakoe-Chiba bounded DTW matrix to find the globally optimal synchronization path, guaranteeing recovery from missed lines or bad translations.
- **Microservice Architecture**: Clean separation of concerns between Node.js (Gateway), Go (Orchestrator), and Python (AI execution).

## Documentation
See the `/docs` folder for detailed system documentation:
- [System Architecture](docs/architecture.md)
- [API & WebSocket Reference](docs/api.md)
- [Algorithms & Pipeline Details](docs/algorithms.md)

## Quick Start

### Prerequisites
- Docker and Docker Compose installed.

### Running the System
```bash
# Build and start all services in detached mode
docker compose up -d --build
```
The system exposes the NestJS API gateway on `http://localhost:3000`.

### Testing via API

1. Upload a video and a bad SRT file:
```bash
curl -X POST http://localhost:3000/sync/upload \
  -F "video=@my_movie.mkv" \
  -F "srt=@bad_subtitles.srt" \
  -F "language=ar"
```
*(This returns a `jobId` immediately).*

2. (Optional) Connect via WebSocket to `http://localhost:3000` and emit `subscribeToJob` with your `jobId` to watch the live progress.

3. Download the finished SRT:
```bash
curl -O -J http://localhost:3000/sync/download/<YOUR_JOB_ID>
```

### Viewing Logs
To see the internal AI pipeline processing the video:
```bash
docker compose logs -f go-worker
docker compose logs -f python-translator
```
