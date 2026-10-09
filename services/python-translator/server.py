import sys
import os
import grpc
from concurrent import futures
import logging
import ssl

# Bypass SSL verification for standard urllib
ssl._create_default_https_context = ssl._create_unverified_context

# Bypass SSL verification for requests (used by Stanza/Argos)
import requests
import warnings
from urllib3.exceptions import InsecureRequestWarning
warnings.simplefilter('ignore', InsecureRequestWarning)
old_request = requests.Session.request
def new_request(self, method, url, **kwargs):
    kwargs['verify'] = False
    return old_request(self, method, url, **kwargs)
requests.Session.request = new_request


# Ensure pb directory is in path
sys.path.append(os.path.join(os.path.dirname(__file__), 'pb'))

import translation_pb2
import translation_pb2_grpc

import argostranslate.package
import argostranslate.translate
import argostranslate.argospm

class TranslationServiceServicer(translation_pb2_grpc.TranslationServiceServicer):
    def __init__(self):
        # Update package list on startup
        logging.info("Updating Argos Translate package list...")
        argostranslate.package.update_package_index()
        self.installed_languages = argostranslate.translate.get_installed_languages()
        
        logging.info("Loading Whisper model (tiny)...")
        import whisper
        self.whisper_model = whisper.load_model("tiny")
        logging.info("Whisper model loaded.")

    def TranscribeAudio(self, request, context):
        audio_path = request.audio_path
        logging.info(f"Received request to transcribe audio: {audio_path}")
        try:
            # Auto-detect the audio language — do NOT hardcode
            result = self.whisper_model.transcribe(audio_path)
            
            detected_lang = result.get("language", "en")
            logging.info(f"Whisper detected language: {detected_lang}")
            
            segments = []
            for seg in result["segments"]:
                segments.append(translation_pb2.WhisperSegment(
                    start=seg["start"],
                    end=seg["end"],
                    text=seg["text"]
                ))
            
            return translation_pb2.TranscribeResponse(
                segments=segments,
                detected_language=detected_lang,
            )
        except Exception as e:
            logging.error(f"Error transcribing audio: {e}")
            context.abort(grpc.StatusCode.INTERNAL, str(e))
        
    def get_or_install_translator(self, from_code, to_code):
        from_lang = next((l for l in self.installed_languages if l.code == from_code), None)
        to_lang = next((l for l in self.installed_languages if l.code == to_code), None)

        if from_lang and to_lang:
            return from_lang.get_translation(to_lang)
            
        logging.info(f"Language pair {from_code}->{to_code} not installed locally. Attempting to install...")
        available_packages = argostranslate.argospm.get_available_packages()
        package_to_install = next(
            (p for p in available_packages if p.from_code == from_code and p.to_code == to_code), None
        )

        if package_to_install:
            package_path = package_to_install.download()
            argostranslate.package.install_from_path(package_path)
            self.installed_languages = argostranslate.translate.get_installed_languages()
            from_lang = next(l for l in self.installed_languages if l.code == from_code)
            to_lang = next(l for l in self.installed_languages if l.code == to_code)
            return from_lang.get_translation(to_lang)
        
        logging.error(f"Translation package {from_code}->{to_code} not found in repository.")
        return None

    def TranslateBlocks(self, request, context):
        source = request.source_language
        target = request.target_language
        
        logging.info(f"Received request to translate {len(request.text_blocks)} blocks from {source} to {target}")
        
        translator = self.get_or_install_translator(source, target)
        if not translator:
            context.abort(grpc.StatusCode.NOT_FOUND, f"Translation pair {source}->{target} is not supported or failed to download.")

        translated_blocks = []
        for block in request.text_blocks:
            # Argos translate works better line-by-line or sentence-by-sentence
            try:
                translated = translator.translate(block)
                translated_blocks.append(translated)
            except Exception as e:
                logging.error(f"Error translating block: {e}")
                translated_blocks.append(block) # Fallback to original

        return translation_pb2.TranslationResponse(translated_blocks=translated_blocks)

def serve():
    logging.basicConfig(level=logging.INFO)
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    translation_pb2_grpc.add_TranslationServiceServicer_to_server(
        TranslationServiceServicer(), server)
    server.add_insecure_port('[::]:50052')
    logging.info("Python Translation Service listening on port 50052")
    server.start()
    server.wait_for_termination()

if __name__ == '__main__':
    serve()
