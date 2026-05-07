# web/Dockerfile
# syntax=docker/dockerfile:1.7

# ---- build stage -----------------------------------------------------
FROM node:20-alpine AS build
WORKDIR /src

# Cache npm install — copy only manifest first.
COPY package.json package-lock.json ./
RUN npm ci

# Build the static bundle.
COPY . .
RUN npm run build

# ---- runtime stage ---------------------------------------------------
FROM nginx:alpine

# Replace default config with our SPA-aware one.
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy the built assets.
COPY --from=build /src/dist /usr/share/nginx/html

EXPOSE 80
# Default nginx CMD already runs the daemon in the foreground.
