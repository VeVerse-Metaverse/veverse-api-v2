############ BUILD ####################
FROM golang:1.19 as builder

#ENV GO111MODULE=off

# Copy service
RUN mkdir -p $GOPATH/src/dev.hackerman.me/artheon/veverse-api
COPY main.go go.mod go.sum $GOPATH/src/dev.hackerman.me/artheon/veverse-api/
COPY aws $GOPATH/src/dev.hackerman.me/artheon/veverse-api/aws/
COPY ai $GOPATH/src/dev.hackerman.me/artheon/veverse-api/ai/
COPY database $GOPATH/src/dev.hackerman.me/artheon/veverse-api/database/
COPY docs $GOPATH/src/dev.hackerman.me/artheon/veverse-api/docs/
COPY handler $GOPATH/src/dev.hackerman.me/artheon/veverse-api/handler/
COPY helper $GOPATH/src/dev.hackerman.me/artheon/veverse-api/helper/
COPY k8s $GOPATH/src/dev.hackerman.me/artheon/veverse-api/k8s/
COPY middleware $GOPATH/src/dev.hackerman.me/artheon/veverse-api/middleware/
COPY model $GOPATH/src/dev.hackerman.me/artheon/veverse-api/model/
COPY oauth $GOPATH/src/dev.hackerman.me/artheon/veverse-api/oauth/
COPY reflect $GOPATH/src/dev.hackerman.me/artheon/veverse-api/reflect/
COPY router $GOPATH/src/dev.hackerman.me/artheon/veverse-api/router/
COPY sessionStore $GOPATH/src/dev.hackerman.me/artheon/veverse-api/sessionStore/
COPY translation $GOPATH/src/dev.hackerman.me/artheon/veverse-api/translation/
COPY validation $GOPATH/src/dev.hackerman.me/artheon/veverse-api/validation/
COPY google $GOPATH/src/dev.hackerman.me/artheon/veverse-api/google/

WORKDIR $GOPATH/src/dev.hackerman.me/artheon/veverse-api
RUN pwd && ls -lah

# Authorize SSH Host
RUN mkdir -p /root/.ssh && \
    chmod 0700 /root/.ssh && \
    ssh-keyscan gitlab.com > /root/.ssh/known_hosts

# Add SSH configuration for the root user for gitlab.com
RUN echo "Host gitlab.com\
        HostName gitlab.com\
        User root\
        IdentityFile ~/.ssh/id_rsa\
        ForwardAgent yes" > ~/.ssh/config

# Add authorized SSH keys for the builder account
ENV PUBLIC_KEY="ssh-rsa XXXXXXX builder@example.com"
ENV PRIVATE_KEY="-----BEGIN OPENSSH PRIVATE KEY-----\n\
xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n\
-----END OPENSSH PRIVATE KEY-----"

# Add the keys and set permissions
RUN echo $PRIVATE_KEY > /root/.ssh/id_rsa && \
    echo $PUBLIC_KEY > /root/.ssh/id_rsa.pub && \
    chmod 600 /root/.ssh/id_rsa && \
    chmod 600 /root/.ssh/id_rsa.pub

# Start ssh-agent
RUN eval `ssh-agent -s` && \
    ssh-add ~/.ssh/id_rsa

# Configure git
RUN git config --global url."git@gitlab.com:".insteadOf "https://gitlab.com/"
RUN git config --global url."git@dev.hackerman.me:".insteadOf "https://dev.hackerman.me/"
RUN ssh-keyscan -t rsa "dev.hackerman.me" >> ~/.ssh/known_hosts
RUN git clone git@dev.hackerman.me:artheon/veverse-shared.git $GOPATH/src/dev.hackerman.me/artheon/veverse-shared
RUN export GOPRIVATE=dev.hackerman.me/artheon/*

# Download required dependencies
RUN go mod tidy

# Build
RUN CGO_ENABLED=0 GO111MODULE=on go build -o /veverse-api

# Remove ssh keys
RUN rm -rf /root/.ssh/

############ RUN ####################
FROM alpine:3.8

COPY .google /root/.google
ENV GOOGLE_APPLICATION_CREDENTIALS /root/.google/credentials.json

COPY --from=builder /veverse-api /usr/local/bin/

RUN ls -lah /usr/local/bin/

WORKDIR /tmp

RUN ls -lah /usr/local/bin/

ENTRYPOINT [ "/usr/local/bin/veverse-api" ]