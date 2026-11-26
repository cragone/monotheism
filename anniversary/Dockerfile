FROM nginx:1.27-alpine
COPY index.html /usr/share/nginx/html/index.html
COPY img /usr/share/nginx/html/img
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
