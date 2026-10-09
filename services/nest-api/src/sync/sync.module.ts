import { Module } from '@nestjs/common';
import { ClientsModule, Transport } from '@nestjs/microservices';
import { join } from 'path';
import { SyncController } from './sync.controller';
import { SyncService } from './sync.service';
import { SyncGateway } from './sync.gateway';

@Module({
  imports: [
    ClientsModule.register([
      {
        name: 'SYNC_PACKAGE',
        transport: Transport.GRPC,
        options: {
          package: 'subtitle_sync',
          protoPath: join(__dirname, '../../../../proto/sync.proto'),
          url: process.env.GO_WORKER_URL || 'localhost:50051',
        },
      },
    ]),
  ],
  controllers: [SyncController],
  providers: [SyncService, SyncGateway],
})
export class SyncModule {}
