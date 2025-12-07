FROM ollama/ollama:0.5.4

# Pre-pull models at build time (optional, increases image size)
# Uncomment to include models in image:
# ARG MODELS="phi3.5:3.8b-mini-instruct-q4_K_M gemma2:2b-instruct-q4_0"
# RUN ollama serve & sleep 5 && \
#     for model in $MODELS; do ollama pull $model; done && \
#     pkill ollama

# Run as non-root user for security (UID 1000)
USER 1000

EXPOSE 11434

CMD ["serve"]
