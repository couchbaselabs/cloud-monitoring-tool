# cloud-monitoring-tool
Tools to monitor cloud usage including Couchbase Cloud infrastructure. This tool generates a cascading report of all 
resources currently being used including:

- Couchbase clouds
- Couchbase cloud clusters
- Cloudformation stacks
- EKS clusters
- EC2 instances
- EBS volumes

## Usage
The tool requires environment variables to be set. These can be set in either `.env` or `.env.test` depending on if you 
want to run with a production configuration or a test configuration.

```
SLACK_CHANNEL_ID=
SLACK_BOT_TOKEN=

AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=
AWS_PRIMARY_ROLE_ARN=

COUCHBASE_CLOUD_ACCESS_KEY=
COUCHBASE_CLOUD_SECRET_KEY=
```

####Run with dev/test configuration
`docker-compose -f "docker-compose.dev.yml" up --build cloud_monitoring_tool`

##Release
To release the tool, it needs to be bundled into a docker image and pushed to a container registry. We use AWS ECR for this:

`./release.sh {{ AWS_ECR_REGISTRY_URL }}`

##Deployment
To deploy in production, use the same registry URL the image was published to. The deployment script configures a cronjob
that schedules the tool to run once a day. The script expects to find a `.env` file in the home directory with the above
variables:

`./deploy.sh $HOME {{ AWS_ECR_REGISTRY_URL }}`

Note: The server must have `aws-cli` set up and configured