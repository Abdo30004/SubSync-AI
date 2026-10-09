import { WebSocketGateway, WebSocketServer, SubscribeMessage, MessageBody, ConnectedSocket } from '@nestjs/websockets';
import { Server, Socket } from 'socket.io';

@WebSocketGateway({ cors: true })
export class SyncGateway {
  @WebSocketServer()
  server: Server;

  @SubscribeMessage('join_job')
  handleJoinJob(@MessageBody() data: { jobId: string }, @ConnectedSocket() client: Socket) {
    console.log(`Client ${client.id} joined room for job ${data.jobId}`);
    client.join(data.jobId);
    return { event: 'joined', data: data.jobId };
  }

  sendChunkToClient(jobId: string, chunk: any) {
    this.server.to(jobId).emit('sync_chunk', chunk);
  }

  sendErrorToClient(jobId: string, error: string) {
    this.server.to(jobId).emit('sync_error', { error });
  }

  sendCompleteToClient(jobId: string) {
    this.server.to(jobId).emit('sync_complete', { status: 'done' });
  }
}
