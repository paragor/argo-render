FROM alpine:3.20.2

RUN apk add --no-cache curl bash

ARG HELM_VERSION=4.0.0
ARG KUSTOMIZE_VERSION=5.8.0

RUN curl -fsSL https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz | tar -xzO linux-amd64/helm > /usr/local/bin/helm \
    && chmod +x /usr/local/bin/helm

RUN curl -fsSL https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz | tar -xzO > /usr/local/bin/kustomize \
    && chmod +x /usr/local/bin/kustomize

WORKDIR /app

COPY argo-render /usr/bin/
ENTRYPOINT ["/usr/bin/argo-render"]
