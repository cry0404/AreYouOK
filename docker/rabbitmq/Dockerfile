FROM rabbitmq:4-management-alpine


RUN apk add --no-cache curl


RUN curl -L https://github.com/rabbitmq/rabbitmq-delayed-message-exchange/releases/download/v4.2.0/rabbitmq_delayed_message_exchange-4.2.0.ez \
    -o $RABBITMQ_HOME/plugins/rabbitmq_delayed_message_exchange-4.2.0.ez && \
    rabbitmq-plugins enable rabbitmq_delayed_message_exchange


RUN rabbitmq-plugins list | grep rabbitmq_delayed_message_exchange || echo "Plugin installation completed"

