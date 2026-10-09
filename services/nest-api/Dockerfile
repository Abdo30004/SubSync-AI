FROM node:20-alpine

WORKDIR /app

# Copy package info
COPY services/nest-api/package.json services/nest-api/package-lock.json* ./
RUN npm install

# Copy proto files (needed for dynamic loading at runtime)
COPY proto /app/proto

# Copy source
COPY services/nest-api/ /app/services/nest-api/
WORKDIR /app/services/nest-api

# Build Nest app
RUN npm run build

CMD ["npm", "run", "start:prod"]
