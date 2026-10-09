# Technical Specifications: Offline AI Subtitle Synchronizer (v2)

## 1. System Overview
A local, offline, microservices-based backend API designed to automatically synchronize out-of-sync subtitle files (`.srt`) with their corresponding video files (`.mp4`, `.mkv`). The system leverages AI (ASR and Translation) to detect actual spoken words and uses a mathematical alignment algorithm to adjust the subtitle timestamps dynamically. It supports "instant play" by streaming synchronized subtitle chunks in real-time as they are processed.

### 1.1 Core Constraints
*   **Environment:** 100% Local (Windows/Linux via Docker).
*   **Connectivity:** Strictly offline after initial setup (optional online translation API fallback).
*   **Cost:** 100% Free and Open-Source Software (FOSS).
*   **Hardware:** Optimized for CPU-only execution.
*   **Performance:** Fast response times using concurrency, VAD, and progressive chunk streaming.
*   **Scope:** Headless API (no built-in web frontend).

---

## 2. System Architecture
The application follows a 3-tier microservices architecture to separate API routing, heavy media processing, and AI text translation, communicating exclusively via high-speed gRPC.

*   **API Gateway (Node.js / NestJS):** Handles HTTP requests, file uploads, job queueing, and real-time WebSocket/SSE streaming to clients.
*   **Processing Engine (Go):** Handles video manipulation, audio extraction, AI inference (Whisper), and subtitle alignment (DTW).
*   **Translation Service (Python):** A lightweight service running an offline translation model to enable cross-language DTW alignment.
*   **Communication (gRPC):** Internal strongly-typed binary communication protocol used between NestJS, Go, and Python.
*   **Storage (Shared Volume):** A locally mounted directory shared between NestJS and Go to avoid network transfer overhead for large video files.

---

## 3. Offline Preparation (Prerequisites)
Before deploying in an offline environment, the following assets must be downloaded and cached locally:
1.  **AI Models (ASR):** `whisper.cpp` model (e.g., `ggml-base.bin` or `ggml-small.bin`).
2.  **AI Models (Translation):** Local Python-compatible translation models (e.g., Argos Translate or MarianMT).
3.  **VAD Model:** Silero VAD (ONNX format) for silence removal.
4.  **Binaries:** `FFmpeg` executable (for audio extraction).
5.  **Dependencies:** `npm install`, `go mod tidy`, and `pip install` must be run once while connected to the internet. 

---

## 4. Processing Pipeline (Data Flow)

The synchronization process follows a strict progressive pipeline:

### Step 1: Audio Extraction
*   **Tool:** FFmpeg (invoked via Go `os/exec`).
*   **Action:** Extracts a lightweight, mono, 16kHz audio track from the video file.

### Step 2: Voice Activity Detection (VAD)
*   **Tool:** Silero VAD.
*   **Action:** Scans the audio and identifies timestamps of human speech, stripping away silences to reduce the AI processing payload.

### Step 3: Text Translation & Alignment Prep
*   **Tool:** Python Translation Service (via gRPC).
*   **Action:** If the uploaded `.srt` language does not match the Whisper target language (usually English), the Go worker sends the `.srt` text to the Python service. Python returns the translated text, which is used purely internally for accurate text-matching.

### Step 4: Concurrent AI Inference & Progressive Alignment
*   **Tool:** `whisper.cpp` and DTW (Dynamic Time Warping).
*   **Action:** The audio is divided into chunks (e.g., 3-minute blocks). Go uses Goroutines to process these chunks through Whisper to generate text/timestamps.
*   **Alignment (Anchor & Interpolate):** For each chunk, DTW compares the Whisper text with the translated `.srt` text. It uses high-confidence matches as "anchors" and linearly interpolates the timestamps for the rest, gracefully handling missing or extra scenes (e.g., Director's Cuts) without failing.

### Step 5: Streaming Delivery
*   **Action:** As soon as Go finishes aligning a chunk, it streams the newly timed, original-language `.srt` blocks back to NestJS via a gRPC stream. NestJS instantly pushes these blocks to the connected client via WebSockets/SSE, enabling "instant play" while background processing continues.

---

## 5. Interface Definition (gRPC)
The communication is strictly defined by Protocol Buffers. 

### 5.1 NestJS <-> Go
```protobuf
syntax = "proto3";
package subtitle_sync;

service SyncService {
  // Uses Server Streaming for progressive delivery
  rpc SyncSubtitles (SyncRequest) returns (stream SyncChunk) {}
}

message SyncRequest {
  string job_id = 1;
  string movie_file_path = 2;
  string original_srt_path = 3;
  string language = 4;
}

message SyncChunk {
  bool success = 1;
  int32 chunk_index = 2;
  bool is_final = 3;
  string srt_content = 4; // The correctly timed subtitles for this chunk
  string error_message = 5;
}
```

### 5.2 Go <-> Python
```protobuf
syntax = "proto3";
package translation;

service TranslationService {
  rpc TranslateBlocks (TranslationRequest) returns (TranslationResponse) {}
}

message TranslationRequest {
  repeated string text_blocks = 1;
  string source_language = 2;
  string target_language = 3;
}

message TranslationResponse {
  repeated string translated_blocks = 1;
}
```

---

## 6. Deployment Configuration (Docker)
The system is orchestrated using `docker-compose`.

*   **`nest-api`:** Exposes port `3000` for REST uploads and WebSocket/SSE streaming.
*   **`go-worker`:** Exposes port `50051` internally for gRPC. Includes FFmpeg and C++ compilers.
*   **`python-translator`:** Exposes port `50052` internally for gRPC. Loads offline translation models.
*   **`shared_data` Volume:** Mounted to both NestJS and Go to instantly access uploaded videos without internal network copying. Python does not require this volume as it receives text directly via gRPC.