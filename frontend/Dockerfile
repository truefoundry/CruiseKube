# Stage 1: Build the React app
FROM public.ecr.aws/docker/library/node:20-alpine AS build

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . ./
RUN npm run build

# Stage 2: Serve the build with nginx
FROM public.ecr.aws/nginx/nginx:alpine

# Copy the build output to nginx's html directory
COPY --from=build /app/build /usr/share/nginx/html

# Copy a custom nginx config if you have one (optional)
# COPY nginx.conf /etc/nginx/nginx.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
