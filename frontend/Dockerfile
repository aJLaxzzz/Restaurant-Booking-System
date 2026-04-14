# Сборка SPA и nginx с демо-статикой (/demo/*).
# docker build -t rbs-web ./frontend

FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm config set registry https://registry.npmjs.org
RUN npm ci
COPY . .
ARG VITE_PUBLIC_ORIGIN=
ENV VITE_PUBLIC_ORIGIN=$VITE_PUBLIC_ORIGIN
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY public/demo /usr/share/nginx/html/demo
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
