import { Injectable, Inject, OnModuleInit } from '@nestjs/common';
import { ClientGrpc } from '@nestjs/microservices';
import { Observable } from 'rxjs';
import { SyncGateway } from './sync.gateway';
import * as fs from 'fs';
import * as path from 'path';

interface SyncServiceGrpc {
  SyncSubtitles(data: { jobId: string; movieFilePath: string; originalSrtPath: string; language: string }): Observable<any>;
}

@Injectable()
export class SyncService implements OnModuleInit {
  private grpcService: SyncServiceGrpc;

  constructor(
    @Inject('SYNC_PACKAGE') private client: ClientGrpc,
    private readonly syncGateway: SyncGateway,
  ) {}

  onModuleInit() {
    this.grpcService = this.client.getService<SyncServiceGrpc>('SyncService');
  }

  startSyncJob(jobId: string, movieFilePath: string, originalSrtPath: string, language: string) {
    console.log(`Starting gRPC stream for job ${jobId}`);
    
    const stream = this.grpcService.SyncSubtitles({
      jobId: jobId,
      movieFilePath: movieFilePath,
      originalSrtPath: originalSrtPath,
      language: language,
    } as any);

    const srtOutputPath = path.join('/app/shared_data', `${jobId}_synced.srt`);
    // Clear file if it exists
    if (fs.existsSync(srtOutputPath)) {
      fs.unlinkSync(srtOutputPath);
    }

    // Subscribe to the stream from Go worker
    stream.subscribe({
      next: (chunk) => {
        console.log(`Received chunk ${chunk.chunkIndex} for job ${jobId}`);
        
        // Append chunk to final SRT file
        if (chunk.srtContent) {
          fs.appendFileSync(srtOutputPath, chunk.srtContent + '\n\n');
        }

        // Push the chunk to the client via WebSocket
        this.syncGateway.sendChunkToClient(jobId, chunk);
      },
      error: (err) => {
        console.error(`Error in stream for job ${jobId}:`, err);
        this.syncGateway.sendErrorToClient(jobId, err.message);
      },
      complete: () => {
        console.log(`Stream completed for job ${jobId}`);
        this.syncGateway.sendCompleteToClient(jobId);
      },
    });
  }
}

