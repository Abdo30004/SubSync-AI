# API Reference

The Subtitle Sync system exposes a REST API and a WebSocket gateway via the NestJS service.

## Base URL
`http://localhost:3000`

---

## 1. Upload & Start Sync

Uploads the video and out-of-sync subtitle file to start the synchronization process.

**Endpoint:** `/sync/upload`  
**Method:** `POST`  
**Content-Type:** `multipart/form-data`

### Request Payload

| Field | Type | Required | Description |
|---|---|---|---|
| `video` | File (`.mp4`, `.mkv`) | **Yes** | The video file containing the spoken audio. |
| `srt` | File (`.srt`) | **Yes** | The out-of-sync subtitle file. |
| `language` | String | No | The language code of the SRT file (e.g., `en`, `ar`, `fr`). Defaults to `en`. |

### Response
Returns immediately with a unique job ID. The actual processing happens asynchronously.

```json
{
  "message": "Processing started",
  "jobId": "78678dcf-97c7-4e92-ac81-2e69697f83d2"
}
```

---

## 2. Download Synced Subtitles

Retrieves the final, synchronized subtitle file.

**Endpoint:** `/sync/download/:jobId`  
**Method:** `GET`

### Parameters
- `jobId`: The UUID returned by the upload endpoint.

### Response
- **200 OK**: Initiates a file download (`<jobId>_synced.srt`).
- **404 Not Found**: The file does not exist (either the job ID is invalid, or the sync hasn't finished yet).

---

## 3. Real-Time Progress (WebSocket)

Clients can connect via WebSockets (Socket.IO) to receive real-time updates as the Go worker processes the file and streams chunks back.

**URL:** `http://localhost:3000`

### Client Events (Emit)

#### `subscribeToJob`
Joins the WebSocket room for a specific job ID to listen for updates.
**Payload:** `jobId` (String)

### Server Events (Listen)

#### `syncUpdate`
Fired continuously as the Go worker completes chunks of processing.
**Payload:**
```json
{
  "success": true,
  "chunkIndex": 2,
  "isFinal": false,
  "srtContent": "42\n00:05:12,000 --> 00:05:15,500\nHello World\n\n...",
  "errorMessage": ""
}
```

#### `syncComplete`
Fired when the entire job is finished and the final SRT is ready for download.
**Payload:** `{ jobId: "..." }`

#### `syncError`
Fired if an error occurs anywhere in the pipeline (FFmpeg extraction, Whisper, DTW, etc).
**Payload:** `{ jobId: "...", message: "Error details..." }`
