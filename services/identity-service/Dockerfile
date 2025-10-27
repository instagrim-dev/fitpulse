# syntax=docker/dockerfile:1

FROM python:3.11-slim
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1
WORKDIR /app
COPY services/identity-service/pyproject.toml /app/
RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir hatchling
COPY services/identity-service/app /app/app
RUN pip install --no-cache-dir .
EXPOSE 8000
ENTRYPOINT ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
