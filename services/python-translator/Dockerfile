FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements and install
COPY services/python-translator/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Download argos models on build to save time later
# (Usually we'd pre-download en-ar or similar, but we'll let argos do it dynamically or pre-download a few basics)
RUN python -c "import argostranslate.package, argostranslate.argospm; argostranslate.argospm.update(); argostranslate.package.install_from_path(argostranslate.argospm.get_available_packages()[0].download())" || true

# Copy proto definitions
COPY proto /app/proto

# Copy service code
COPY services/python-translator/ /app/services/python-translator/

# Generate Python gRPC stubs
RUN mkdir -p /app/services/python-translator/pb
RUN python -m grpc_tools.protoc -I/app/proto --python_out=/app/services/python-translator/pb --grpc_python_out=/app/services/python-translator/pb /app/proto/translation.proto

# Make the pb folder a python module
RUN touch /app/services/python-translator/pb/__init__.py

WORKDIR /app/services/python-translator
CMD ["python", "server.py"]
