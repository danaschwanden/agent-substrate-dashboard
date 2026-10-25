# syntax=docker/dockerfile:1.12
FROM node:22.17.0-alpine3.22 AS build

WORKDIR /src
COPY package.json package-lock.json ./
COPY frontend/package.json frontend/package.json
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY frontend/ frontend/
COPY api/fixtures/ api/fixtures/
RUN npm --workspace frontend run build

FROM nginxinc/nginx-unprivileged:1.31.7-alpine3.24
ARG VERSION=development
LABEL org.opencontainers.image.title="Agent Substrate Dashboard Frontend" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.licenses="Apache-2.0"
ENV DASHBOARD_BACKEND_URL=http://agent-substrate-dashboard-backend:8080
COPY frontend/nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /src/frontend/dist/ /usr/share/nginx/html/
EXPOSE 8080
