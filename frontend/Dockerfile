# syntax=docker/dockerfile:1.7

FROM nginx:1.29-alpine AS runtime

COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY public /usr/share/nginx/html
COPY --from=openapi openapi.yaml /usr/share/nginx/html/docs/openapi.yaml

EXPOSE 80
