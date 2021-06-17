package monitoring

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/couchbaselabs/couchbase-cloud-go-client/couchbasecloud"
	"log"
	"math"
	"os"
	"time"
)

const cbcApiAccessKeyEnv = "COUCHBASE_CLOUD_ACCESS_KEY"
const cbcApiSecretKeyEnv = "COUCHBASE_CLOUD_SECRET_KEY"
const awsPrimaryRoleArnEnv = "AWS_PRIMARY_ROLE_ARN"

const awsSessionName = "cloud-monitoring-tool"
const ec2EksClusterNameTag = "cluster"
const cloudformationEc2StackResourceId = "AWS::EC2::Instance"

var regions = []string{
	"us-east-1", "us-east-2", "us-west-2", "eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
	"ca-central-1", "us-west-1", "ap-south-1", "ap-northeast-2", "ap-southeast-1", "ap-southeast-2", "ap-northeast-1",
}

func processEC2Claims(ctx *RegionalCloudContext) {
	ebsCountBefore := len(ctx.EBSVolumes)

	for _, ec2Instance := range ctx.EC2Instances {
		for _, blockDevice := range ec2Instance.InstanceBlockDeviceMappings {
			ec2VolumeId := *blockDevice.Ebs.VolumeId

			if ebsVolume, ok := ctx.EBSVolumes[ec2VolumeId]; ok {
				ec2Instance.Claim(ctx, ebsVolume)
			}
		}
	}

	ebsCountAfter := len(ctx.EBSVolumes)
	log.Printf("Processed EC2 claims (%d EBS volumes)", ebsCountBefore - ebsCountAfter)
}

func processCouchbaseCloudClusterClaims(ctx *RegionalCloudContext) {
	ec2InstancesByClusterId := ctx.GetEC2InstancesByClusterId()
	ec2CountBefore := len(ctx.EC2Instances)

	for _, couchbaseCloudCluster := range ctx.CouchbaseCloudClusters {
		clusterId := couchbaseCloudCluster.ID

		if ec2Instances, ok := ec2InstancesByClusterId[clusterId]; ok {
			for _, ec2Instance := range ec2Instances {
				if eksClusterName, ok := ec2Instance.Tags[ec2EksClusterNameTag]; ok {
					couchbaseCloudCluster.EKSClusterName = eksClusterName
				}

				couchbaseCloudCluster.Claim(ctx, ec2Instance)
			}
		}
	}

	ec2CountAfter := len(ctx.EC2Instances)
	log.Printf("Processed Couchbase Cloud Cluster claims (%d EC2 instances)", ec2CountBefore - ec2CountAfter)
}

func processEKSClusterClaims(ctx *RegionalCloudContext) {
	couchbaseCloudClustersByEKSName := ctx.GetCouchbaseCloudClustersByEKSName()
	ec2InstancesBySubnetId := ctx.GetEC2InstancesBySubnetId()
	cbcCountBefore := len(ctx.CouchbaseCloudClusters)
	ec2CountBefore := len(ctx.EC2Instances)

	for _, eksCluster := range ctx.EKSClusters {
		eksName := eksCluster.Name

		if couchbaseClusters, ok := couchbaseCloudClustersByEKSName[eksName]; ok {
			for _, couchbaseCluster := range couchbaseClusters {
				eksCluster.Claim(ctx, couchbaseCluster)
			}
		}

		// Linking EKS clusters to EC2 instances directly and reliably is not possible without K8S permissions.
		// Assume all EC2 instances within subnets associated with EKS cluster belong to it
		for _, eksSubnetId := range eksCluster.Subnets {
			if ec2Instances, ok := ec2InstancesBySubnetId[*eksSubnetId]; ok {
				for _, ec2Instance := range ec2Instances {
					eksCluster.Claim(ctx, ec2Instance)
				}
			}
		}

	}

	cbcCountAfter := len(ctx.CouchbaseCloudClusters)
	ec2CountAfter := len(ctx.EC2Instances)
	log.Printf("Processed EKS Cluster claims (%d EC2 instances, %d CBC clusters)", ec2CountBefore - ec2CountAfter, cbcCountBefore - cbcCountAfter)
}

func processCloudformationStackClaims(ctx *RegionalCloudContext) {
	ec2CountBefore := len(ctx.EC2Instances)

	for _, cloudformationStack := range ctx.CloudFormationStacks {
		for _, stackResource := range cloudformationStack.StackResourceList {
			switch *stackResource.ResourceType {
			case cloudformationEc2StackResourceId:
				if ec2Instance, ok := ctx.EC2Instances[*stackResource.PhysicalResourceId]; ok {
					cloudformationStack.Claim(ctx, ec2Instance)
				}
			}
		}
	}

	ec2CountAfter := len(ctx.EC2Instances)
	log.Printf("Processed Cloudformation Stack claims (%d EC2 instances)", ec2CountBefore - ec2CountAfter)
}

func processCouchbaseCloudClaims(ctx *RegionalCloudContext) {
	eksClustersByCloudId := ctx.GetEKSClustersByCloudId()
	cloudformationStacksByCloudId := ctx.GetCloudformationStacksByCloudId()
	eksCountBefore := len(ctx.EKSClusters)
	cfStackCountBefore := len(ctx.CloudFormationStacks)

	for _, couchbaseCloud := range ctx.CouchbaseClouds {
		if eksCluster, ok := eksClustersByCloudId[couchbaseCloud.ID]; ok {
			couchbaseCloud.Claim(ctx, eksCluster)
		}

		if cloudformationStack, ok := cloudformationStacksByCloudId[couchbaseCloud.ID]; ok {
			couchbaseCloud.Claim(ctx, cloudformationStack)
		}
	}

	eksCountAfter := len(ctx.EKSClusters)
	cfStackCountAfter := len(ctx.CloudFormationStacks)
	log.Printf("Processed Couchbase Cloud claims (%d EKS clusters, %d CF stacks)", eksCountBefore - eksCountAfter, cfStackCountBefore - cfStackCountAfter)
}

func assumeRole(role string, sess *session.Session, roleSessionName string) (*sts.Credentials, error) {
	stsSvc := sts.New(sess)
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(role),
		RoleSessionName: aws.String(roleSessionName),
		DurationSeconds: aws.Int64(3600),
	}

	result, err := stsSvc.AssumeRole(input)
	if err != nil {
		return nil, err
	}

	return result.Credentials, err
}

func getCallerId(s *session.Session) (*string, error) {
	iamSvc := iam.New(s)
	input := &iam.GetUserInput{}
	result, err := iamSvc.GetUser(input)
	if err != nil {
		return nil, fmt.Errorf("unable to get caller ID %w", err)
	}

	return result.User.UserName, err
}

func getCouchbaseClouds(client *couchbasecloud.CouchbaseCloudClient) (map[string]*CouchbaseCloud, error) {
	clouds := map[string]*CouchbaseCloud{}
	page := 1
	lastPage := math.MaxInt16

	for ok := true; ok; ok = page <= lastPage {
		listCloudsResponse, err := client.ListClouds(&couchbasecloud.ListCloudsOptions{Page: page, PerPage: 10})

		if err != nil {
			return nil, err
		}

		for _, cloudResponse := range listCloudsResponse.Data {
			cloud := NewCouchbaseCloud()
			cloud.ID = cloudResponse.Id
			cloud.Name = cloudResponse.Name
			cloud.Region = cloudResponse.Region
			cloud.Provider = cloudResponse.Provider
			cloud.Status = cloudResponse.Status
			cloud.VirtualNetworkCIDR = cloudResponse.VirtualNetworkCIDR
			cloud.VirtualNetworkID = cloudResponse.VirtualNetworkID
			clouds[cloud.ID] = cloud
		}

		lastPage = listCloudsResponse.Cursor.Pages.Last
		page++
	}

	log.Printf("Found %d Couchbase Clouds", len(clouds))
	return clouds, nil
}

func getCouchbaseClusters(client *couchbasecloud.CouchbaseCloudClient) (map[string]*CouchbaseCloudCluster, error) {
	clusters := map[string]*CouchbaseCloudCluster{}
	page := 1
	lastPage := math.MaxInt16

	for ok := true; ok; ok = page <= lastPage {
		listClustersResponse, err := client.ListClusters(&couchbasecloud.ListClustersOptions{Page: page, PerPage: 10})

		if err != nil {
			return nil, err
		}

		for _, clusterResponse := range listClustersResponse.Data {
			cluster := NewCouchbaseCloudCluster()
			cluster.ID = clusterResponse.Id
			cluster.Name = clusterResponse.Name
			cluster.NodeCount = clusterResponse.Nodes
			cluster.Services = clusterResponse.Services
			clusters[cluster.ID] = cluster
		}

		lastPage = listClustersResponse.Cursor.Pages.Last
		page++
	}

	log.Printf("Found %d Couchbase Clusters", len(clusters))
	return clusters, nil
}

func getEBSVolumes(ec2Svc *ec2.EC2, region string) (map[string]EBSVolume, error) {
	ebsVolumes := map[string]EBSVolume{}

	input := &ec2.DescribeVolumesInput{
		MaxResults: aws.Int64(100),
	}

	err := ec2Svc.DescribeVolumesPages(input, func(page *ec2.DescribeVolumesOutput, lastPage bool) bool {
		for _, volume := range page.Volumes {
			id := *volume.VolumeId
			ebsVolume := NewEBSVolume()
			ebsVolume.ID = id
			ebsVolume.Region = region

			if volume.CreateTime != nil {
				ebsVolume.CreatedAt = *volume.CreateTime
			}

			if volume.Size != nil {
				ebsVolume.SizeGiB = *volume.Size
			}

			if volume.State != nil {
				ebsVolume.State = *volume.State
			}

			ebsVolume.Type = volume.VolumeType

			volumeTags := map[string]string{}

			for _, tag := range volume.Tags {
				if tag == nil || tag.Key == nil {
					continue
				}

				volumeTags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
			}

			ebsVolume.Tags = volumeTags

			if name, ok := ebsVolume.Tags["Name"]; ok {
				ebsVolume.Name = name
			}

			ebsVolumes[id] = *ebsVolume

		}
		return !lastPage
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get EBS volumes %w", err)
	}

	log.Printf("Found %d EBS volumes", len(ebsVolumes))
	return ebsVolumes, nil
}

func getEC2Instances(ec2Service *ec2.EC2, region string) (map[string]EC2Instance, error) {
	ec2Instances := map[string]EC2Instance{}

	input := &ec2.DescribeInstancesInput{
		MaxResults: aws.Int64(100),
	}

	err := ec2Service.DescribeInstancesPages(input, func(output *ec2.DescribeInstancesOutput, lastPage bool) bool {
		for _, reservation := range output.Reservations {
			for _, instanceDescription := range reservation.Instances {
				id := *instanceDescription.InstanceId
				ec2Instance := NewEC2Instance()
				ec2Instance.ID = id
				ec2Instance.Region = region
				ec2Instance.InstanceBlockDeviceMappings = instanceDescription.BlockDeviceMappings

				if instanceDescription.SubnetId != nil {
					ec2Instance.SubnetID = *instanceDescription.SubnetId
				}
				if instanceDescription.InstanceType != nil {
					ec2Instance.InstanceType = *instanceDescription.InstanceType
				}
				if instanceDescription.KeyName != nil {
					ec2Instance.KeyName = *instanceDescription.KeyName
				}
				if instanceDescription.Platform != nil {
					ec2Instance.Platform = *instanceDescription.Platform
				}
				if instanceDescription.LaunchTime != nil {
					ec2Instance.CreatedAt = *instanceDescription.LaunchTime
				}

				ec2Tags := map[string]string{}

				for _, tag := range instanceDescription.Tags {
					if tag == nil || tag.Key == nil {
						continue
					}

					ec2Tags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
				}

				ec2Instance.Tags = ec2Tags

				if name, ok := ec2Instance.Tags["Name"]; ok {
					ec2Instance.Name = name
				}

				ec2Instances[id] = *ec2Instance
			}
		}
		return !lastPage
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get instances in the VPC %w", err)
	}

	log.Printf("Found %d EC2 instances", len(ec2Instances))
	return ec2Instances, nil
}

func getEKSClusters(sess *session.Session, awsCredentials *sts.Credentials, region string) (map[string]EKSCluster, error) {
	eksClustersMap := map[string]EKSCluster{}

	eksService := eks.New(sess, &aws.Config{
		Credentials: credentials.NewStaticCredentials(
			aws.StringValue(awsCredentials.AccessKeyId),
			aws.StringValue(awsCredentials.SecretAccessKey),
			aws.StringValue(awsCredentials.SessionToken),
		),
		Region: aws.String(region)})

	clusterListInput := &eks.ListClustersInput{}
	response, err := eksService.ListClusters(clusterListInput)

	if err != nil {
		return nil, fmt.Errorf("unable to get EKS clusters for %s: %s", region, err)
	}

	clusters := response.Clusters

	for idx, cluster := range clusters {
		clusterDescriptionInput := &eks.DescribeClusterInput{
			Name: aws.String(*cluster),
		}

		clusterDescription, err := eksService.DescribeCluster(clusterDescriptionInput)
		if err != nil {
			log.Printf("Unable to describe cluster %d in %s", idx, region)
			continue
		}

		now := time.Now()
		eksCluster := NewEKSCluster()

		if clusterDescription.Cluster.Name != nil {
			eksCluster.Name = *clusterDescription.Cluster.Name
		}

		if clusterDescription.Cluster.ResourcesVpcConfig.VpcId != nil {
			eksCluster.VpcId = *clusterDescription.Cluster.ResourcesVpcConfig.VpcId
		}

		if clusterDescription.Cluster.CreatedAt != nil {
			eksCluster.Age = now.Sub(*clusterDescription.Cluster.CreatedAt)
			eksCluster.CreatedAt = *clusterDescription.Cluster.CreatedAt
		}

		eksCluster.Subnets = clusterDescription.Cluster.ResourcesVpcConfig.SubnetIds

		eksTags := map[string]string{}

		for key, value := range clusterDescription.Cluster.Tags {
			eksTags[key] = *value
		}

		eksCluster.Tags = eksTags
		eksClustersMap[eksCluster.Name] = *eksCluster

		log.Printf("Found EKS cluster: %s in %s", eksCluster.Name, eksCluster.VpcId)
	}

	return eksClustersMap, nil
}

func getCloudformationStacks(sess *session.Session, awsCredentials *sts.Credentials, region string) (map[string]CloudformationStack, error) {
	cloudformationStacksMap := map[string]CloudformationStack{}

	cloudformationService := cloudformation.New(sess, &aws.Config{
		Credentials: credentials.NewStaticCredentials(
			aws.StringValue(awsCredentials.AccessKeyId),
			aws.StringValue(awsCredentials.SecretAccessKey),
			aws.StringValue(awsCredentials.SessionToken),
		),
		Region: aws.String(region)})

	input := &cloudformation.DescribeStacksInput{}
	result, err := cloudformationService.DescribeStacks(input)

	if err != nil {
		return nil, fmt.Errorf("unable to get Cloudformation stacks for %s: %s", region, err)
	}

	for _, stackDescription := range result.Stacks {
		cloudformationStack := NewCloudFormationStack()

		if stackDescription.StackId != nil {
			cloudformationStack.ID = *stackDescription.StackId
		}

		if stackDescription.StackName != nil {
			cloudformationStack.Name = *stackDescription.StackName
		}

		if stackDescription.CreationTime != nil {
			cloudformationStack.CreatedAt = *stackDescription.CreationTime
			cloudformationStack.CreationDuration = time.Now().Sub(*stackDescription.CreationTime)
		}

		cloudformationStack.Region = region
		cloudformationStack.Parameters = getCloudformationStackParameters(stackDescription)

		stackResourceList, err := getCloudformationStackResourceList(cloudformationService, cloudformationStack.Name)

		if err != nil {
			log.Println(err)
		} else {
			cloudformationStack.StackResourceList = stackResourceList
		}

		cloudformationStacksMap[cloudformationStack.ID] = *cloudformationStack
	}

	log.Printf("Found %d Cloudformation stacks", len(cloudformationStacksMap))
	return cloudformationStacksMap, nil
}

func getCloudformationStackResourceList(cloudformationService *cloudformation.CloudFormation, cloudformationStackName string) ([]*cloudformation.StackResourceSummary, error) {
	var stackResourceSummaries []*cloudformation.StackResourceSummary

	listStacksInput := &cloudformation.ListStackResourcesInput{
		StackName: &cloudformationStackName,
	}

	err := cloudformationService.ListStackResourcesPages(listStacksInput, func(listStacksOutput *cloudformation.ListStackResourcesOutput, lastPage bool) bool {
		for _, stackResourceSummary := range listStacksOutput.StackResourceSummaries {
			stackResourceSummaries = append(stackResourceSummaries, stackResourceSummary)
		}
		return !lastPage
	})

	if err != nil {
		return nil, fmt.Errorf("unable to list stack resource summaries for Cloudformation stack %s: %s", cloudformationStackName, err)
	}

	return stackResourceSummaries, nil
}

func getCloudformationStackParameters(stackDescription *cloudformation.Stack) map[string]string {
	parameters := map[string]string{}

	for _, parameter := range stackDescription.Parameters {
		if parameter == nil || parameter.ParameterKey == nil {
			continue
		}

		parameters[aws.StringValue(parameter.ParameterKey)] = aws.StringValue(parameter.ParameterValue)
	}
	return parameters
}

func getEC2Service(sess *session.Session, awsCredentials *sts.Credentials, region string) *ec2.EC2 {
	ec2Svc := ec2.New(sess, &aws.Config{
		Credentials: credentials.NewStaticCredentials(
			aws.StringValue(awsCredentials.AccessKeyId),
			aws.StringValue(awsCredentials.SecretAccessKey),
			aws.StringValue(awsCredentials.SessionToken),
		),
		Region: aws.String(region)})
	return ec2Svc
}

// Set the Couchbase Cloud resources belonging to this context and remove them from the resources remaining to be seen
func cleanup(ctx *RegionalCloudContext, couchbaseClouds map[string]*CouchbaseCloud, couchbaseClusters map[string]*CouchbaseCloudCluster) {
	couchbaseCloudsSeen := map[string]*CouchbaseCloud{}
	couchbaseCloudClustersSeen := map[string]*CouchbaseCloudCluster{}

	for _, cloud := range ctx.CouchbaseClouds {
		if cloud.Seen {
			couchbaseCloudsSeen[cloud.ID] = cloud
			delete(couchbaseClouds, cloud.ID)
		}
	}

	for _, cluster := range ctx.CouchbaseCloudClusters {
		if cluster.Seen {
			couchbaseCloudClustersSeen[cluster.ID] = cluster
			delete(couchbaseClusters, cluster.ID)
		}
	}

	ctx.CouchbaseClouds = couchbaseCloudsSeen
	ctx.CouchbaseCloudClusters = couchbaseCloudClustersSeen
}

func AnalyseAWS() (*GlobalCloudContext, error) {
	client := couchbasecloud.NewClient(os.Getenv(cbcApiAccessKeyEnv), os.Getenv(cbcApiSecretKeyEnv))
	couchbaseCloudsRemaining, err := getCouchbaseClouds(client)

	if err != nil {
		return nil, fmt.Errorf("unable to retrieve couchbase Couchbase Clouds: %s", err)
	}

	couchbaseClustersRemaining, err := getCouchbaseClusters(client)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve couchbase Couchbase clusters: %s", err)
	}

	globalCtx := NewGlobalCloudContext()

	awsSession, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("unable to create AWS session: %s", err)
	}

	callerId, err := getCallerId(awsSession)
	if err != nil {
		return nil, fmt.Errorf("unable to get AWS Caller ID: %s", err)
	}

	awsCredentials, err := assumeRole(os.Getenv(awsPrimaryRoleArnEnv), awsSession, fmt.Sprintf("%s-%v", awsSessionName, callerId))
	if err != nil {
		return nil, fmt.Errorf("unable to assume AWS role: %s", err)
	}

	for _, region := range regions {
		log.Printf("Analysing AWS %s", region)

		ec2Service := getEC2Service(awsSession, awsCredentials, region)

		ebsVolumes, err := getEBSVolumes(ec2Service, region)
		if err != nil {
			return nil, fmt.Errorf("unable to get EBS Volumes in %s: %s", region, err)
		}

		ec2Instances, err := getEC2Instances(ec2Service, region)
		if err != nil {
			return nil, fmt.Errorf("unable to get EC2 Instances in %s: %s", region, err)
		}

		eksClusters, err := getEKSClusters(awsSession, awsCredentials, region)
		if err != nil {
			return nil, fmt.Errorf("unable to get EKS clusters in %s: %s", region, err)
		}

		cloudformationStacks, err := getCloudformationStacks(awsSession, awsCredentials, region)
		if err != nil {
			return nil, fmt.Errorf("unable to get Cloudformation stacks in %s: %s", region, err)
		}

		ctx := NewRegionalCloudContext(region)

		ctx.EBSVolumes = ebsVolumes
		ctx.EC2Instances = ec2Instances
		ctx.CouchbaseCloudClusters = couchbaseClustersRemaining
		ctx.EKSClusters = eksClusters
		ctx.CloudFormationStacks = cloudformationStacks
		ctx.CouchbaseClouds = couchbaseCloudsRemaining

		processEC2Claims(ctx)
		processCouchbaseCloudClusterClaims(ctx)
		processEKSClusterClaims(ctx)
		processCloudformationStackClaims(ctx)
		processCouchbaseCloudClaims(ctx)
		cleanup(ctx, couchbaseCloudsRemaining, couchbaseClustersRemaining)

		globalCtx.Add(*ctx)
	}

	return globalCtx, nil
}
