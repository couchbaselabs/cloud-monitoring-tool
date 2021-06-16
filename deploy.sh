USER_HOME=$1
AWS_ECR_URI=$2
NAME="cloud monitoring tool"

if [ $# -ne 2 ]; then
    echo "User home is required as the first argument and AWS ECR URI as the second"
    exit 1
fi

LOGS_DIR="$USER_HOME/cloud_monitoring_tool.log"

echo "Deploying $NAME"

if aws ecr get-login-password --region "us-west-2" | docker login --username AWS --password-stdin "$AWS_ECR_URI" > /dev/null 2>&1; then
    echo "Logged into ECR $AWS_ECR_URI"
else
    echo "Unable to log into ECR $AWS_ECR_URI"
    exit 1
fi

echo "Pulling latest version of $NAME from $AWS_ECR_URI"

if docker pull "$AWS_ECR_URI":latest; then
    echo "Pulled $AWS_ECR_URI latest"
else
    echo "Unable to pull $AWS_ECR_URI:latest"
    exit 1
fi

ENV_FILE=$USER_HOME/.env
if test -f "$ENV_FILE"; then
    echo "Using $ENV_FILE environment variables"
else
    echo "Unable to find environment variables file $ENV_FILE"
    exit 1
fi

echo "Running $NAME >> $LOGS_DIR"

if docker run --env-file "$ENV_FILE" "$AWS_ECR_URI":latest > "$LOGS_DIR" 2>&1; then
  echo "Finished running $NAME"
else
  echo "Something went wrong when running $NAME"
  exit 1
fi

echo "Clearing crontab"
crontab -r

echo "Adding scheduled cronjob"

echo "0 13 * * * docker run --env-file $ENV_FILE $AWS_ECR_URI:latest >> $LOGS_DIR 2>&1" | crontab -

echo "Deployment of $NAME is complete!"