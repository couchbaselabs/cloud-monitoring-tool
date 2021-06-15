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

####Run with test configuration
`docker-compose -f "docker-compose.test.yml" up --build cloud_monitoring_tool`

####Run with production configuration
`docker-compose up --build cloud_monitoring_tool`
