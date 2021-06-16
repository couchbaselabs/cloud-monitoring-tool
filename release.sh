AWS_ECR_URI=$1
NAME="cloud monitoring tool"

if [ $# -eq 0 ]; then
    echo "The AWS ECR URI is required as the first argument"
    exit 1
fi

echo "Releasing $NAME"

if aws ecr get-login-password | docker login --username AWS --password-stdin "$AWS_ECR_URI" > /dev/null 2>&1; then
    echo "Logged into ECR $AWS_ECR_URI"
else
    echo "Unable to log into ECR $AWS_ECR_URI"
fi

echo "Building $NAME image"

if docker build -t "$AWS_ECR_URI":latest .; then
  echo "Built $NAME docker image"
else
  echo "Unable to build $NAME docker image"
fi

echo "Pushing $NAME docker image"

if docker push "$AWS_ECR_URI":latest; then
  echo "Published $NAME docker image to $AWS_ECR_URI"
else
  echo "Failed to publish $NAME docker image to $AWS_ECR_URI"
fi
