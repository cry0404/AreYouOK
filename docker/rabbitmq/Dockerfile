FROM docker.1ms.run/rabbitmq:4-management-alpine


# 使用阿里云镜像源加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 安装 curl 用于下载插件

RUN apk add --no-cache curl

# 下载并安装延迟消息插件
RUN curl -L https://github.com/rabbitmq/rabbitmq-delayed-message-exchange/releases/download/v4.2.0/rabbitmq_delayed_message_exchange-4.2.0.ez \
    -o /opt/rabbitmq/plugins/rabbitmq_delayed_message_exchange-4.2.0.ez && \
    rabbitmq-plugins enable rabbitmq_delayed_message_exchange

# 验证插件安装
RUN rabbitmq-plugins list | grep rabbitmq_delayed_message_exchange

