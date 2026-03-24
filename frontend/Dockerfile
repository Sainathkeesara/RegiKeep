# ── Build stage ───────────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app
COPY package.json package-lock.json* bun.lockb* ./
RUN npm install --frozen-lockfile 2>/dev/null || npm install

COPY . .

# Build arg lets docker-compose inject the backend URL at build time
ARG VITE_SUPABASE_URL=http://localhost:8080
ENV VITE_SUPABASE_URL=$VITE_SUPABASE_URL

RUN npm run build

# ── Serve stage ───────────────────────────────────────────────────────────────
FROM nginx:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
