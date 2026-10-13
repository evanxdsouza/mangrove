# The public landing page: static files served by an unprivileged nginx on
# port 8080. Build from this directory (site/ is the build context).
FROM nginxinc/nginx-unprivileged:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY index.html favicon.svg install.md llms.txt /usr/share/nginx/html/
COPY fonts/ /usr/share/nginx/html/fonts/
EXPOSE 8080
