import { Controller, Post, Body, UploadedFiles, UseInterceptors, BadRequestException, Get, Param, Res, NotFoundException } from '@nestjs/common';
import { FileFieldsInterceptor } from '@nestjs/platform-express';
import { SyncService } from './sync.service';
import { diskStorage } from 'multer';
import { v4 as uuidv4 } from 'uuid';
import * as path from 'path';
import * as fs from 'fs';
import { Response } from 'express';
import { ApiConsumes, ApiBody, ApiParam } from '@nestjs/swagger';

@Controller('sync')
export class SyncController {
  constructor(private readonly syncService: SyncService) {}

  @Post('upload')
  @ApiConsumes('multipart/form-data')
  @ApiBody({
    schema: {
      type: 'object',
      properties: {
        video: {
          type: 'string',
          format: 'binary',
          description: 'The video file (.mp4, .mkv)',
        },
        srt: {
          type: 'string',
          format: 'binary',
          description: 'The out-of-sync subtitle file (.srt)',
        },
        language: {
          type: 'string',
          default: 'en',
          description: 'Target language of the subtitles (e.g., "en", "ar")',
        },
      },
    },
  })
  @UseInterceptors(
    FileFieldsInterceptor([
      { name: 'video', maxCount: 1 },
      { name: 'srt', maxCount: 1 },
    ], {
      storage: diskStorage({
        destination: '/app/shared_data', // Absolute mapped volume
        filename: (req, file, cb) => {
          const uniqueSuffix = Date.now() + '-' + Math.round(Math.random() * 1e9);
          cb(null, file.fieldname + '-' + uniqueSuffix + path.extname(file.originalname));
        },
      }),
    }),
  )
  async uploadFiles(
    @UploadedFiles() files: { video?: Express.Multer.File[]; srt?: Express.Multer.File[] },
    @Body('language') language: string,
  ) {
    if (!files.video || !files.srt) {
      throw new BadRequestException('Both video and srt files are required.');
    }

    const jobId = uuidv4();
    const videoPath = files.video[0].filename; // relative to shared_data
    const srtPath = files.srt[0].filename;     // relative to shared_data

    // Start the gRPC streaming process asynchronously
    this.syncService.startSyncJob(jobId, videoPath, srtPath, language || 'en');

    // Instantly return the job ID so the client can connect to WebSockets
    return {
      message: 'Processing started',
      jobId: jobId,
    };
  }

  @Get('download/:jobId')
  @ApiParam({ name: 'jobId', required: true, description: 'The Job ID returned by the upload endpoint' })
  downloadSrt(@Param('jobId') jobId: string, @Res() res: Response) {
    const srtPath = path.join('/app/shared_data', `${jobId}_synced.srt`);
    if (!fs.existsSync(srtPath)) {
      throw new NotFoundException('Synced SRT file not found or not finished yet.');
    }
    return res.download(srtPath);
  }
}
