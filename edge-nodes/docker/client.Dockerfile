FROM nginx:alpine

RUN rm /etc/nginx/conf.d/default.conf
# O nginx.conf deve estar na raiz de edge-nodes/
COPY nginx.conf /etc/nginx/conf.d/

# A pasta client deve estar na raiz de edge-nodes/
COPY ./client /usr/share/nginx/html

EXPOSE 80