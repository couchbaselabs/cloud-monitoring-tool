package monitoring

import (
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"time"
)

type CloudResourceClaimer interface {
	Claim(ctx *RegionalCloudContext, resource interface{}) bool
}

type CloudResource struct {
	ID         string
	Name       string
	LaunchedBy string
	Region     string
	Tags       map[string]string
	CreatedAt  time.Time
}

type EBSVolume struct {
	CloudResource
	Type    *string
	SizeGiB int64
	State   string
}

type EC2Instance struct {
	CloudResource
	SubnetID                    string
	InstanceType                string
	KeyName                     string
	Platform                    string
	EBSVolumes                  map[string]EBSVolume
	InstanceBlockDeviceMappings []*ec2.InstanceBlockDeviceMapping
}

type CouchbaseCloudCluster struct {
	CloudResource
	NodeCount      int
	Services       []string
	EKSClusterName string
	EC2Instances   map[string]EC2Instance
	Seen           bool
}

type EKSCluster struct {
	CloudResource
	VpcId                  string
	Age                    time.Duration
	Subnets                []*string
	EC2Instances           map[string]EC2Instance
	CouchbaseCloudClusters map[string]CouchbaseCloudCluster
}

type CloudformationStack struct {
	CloudResource
	CreationDuration  time.Duration
	Parameters        map[string]string
	StackResourceList []*cloudformation.StackResourceSummary
	EC2Instances      map[string]EC2Instance
	EKSClusters       map[string]EKSCluster
}

type CouchbaseCloud struct {
	CloudResource
	Provider            string
	Status              string
	VirtualNetworkCIDR  string
	VirtualNetworkID    string
	EKSClusters         map[string]EKSCluster
	CloudFormationStack *CloudformationStack
	Seen                bool
}

type VPC struct {
	CloudResource
}

func NewEBSVolume() *EBSVolume {
	return &EBSVolume{}
}

func NewEC2Instance() *EC2Instance {
	return &EC2Instance{
		EBSVolumes: make(map[string]EBSVolume),
	}
}

func NewCouchbaseCloudCluster() *CouchbaseCloudCluster {
	return &CouchbaseCloudCluster{
		EC2Instances: make(map[string]EC2Instance),
	}
}

func NewEKSCluster() *EKSCluster {
	return &EKSCluster{
		EC2Instances:           make(map[string]EC2Instance),
		CouchbaseCloudClusters: make(map[string]CouchbaseCloudCluster),
	}
}

func NewCloudFormationStack() *CloudformationStack {
	return &CloudformationStack{
		EC2Instances: make(map[string]EC2Instance),
		EKSClusters:  make(map[string]EKSCluster),
	}
}

func NewCouchbaseCloud() *CouchbaseCloud {
	return &CouchbaseCloud{
		EKSClusters: make(map[string]EKSCluster),
	}
}

func (ec2Instance *EC2Instance) Claim(ctx *RegionalCloudContext, resource interface{}) {
	switch resource.(type) {
	case EBSVolume:
		ctx.Claim(resource)
		ebsVolume := resource.(EBSVolume)
		ec2Instance.EBSVolumes[ebsVolume.ID] = ebsVolume
	}
}

func (couchbaseCloudCluster *CouchbaseCloudCluster) Claim(ctx *RegionalCloudContext, resource interface{}) {
	switch resource.(type) {
	case EC2Instance:
		ctx.Claim(resource)
		ec2Instance := resource.(EC2Instance)
		couchbaseCloudCluster.EC2Instances[ec2Instance.ID] = ec2Instance
		couchbaseCloudCluster.Seen = true
	}
}

func (eksCluster *EKSCluster) Claim(ctx *RegionalCloudContext, resource interface{}) {
	switch resource.(type) {
	case EC2Instance:
		ctx.Claim(resource)
		ec2Instance := resource.(EC2Instance)
		eksCluster.EC2Instances[ec2Instance.ID] = ec2Instance
	case CouchbaseCloudCluster:
		ctx.Claim(resource)
		couchbaseCloudCluster := resource.(CouchbaseCloudCluster)
		eksCluster.CouchbaseCloudClusters[couchbaseCloudCluster.ID] = couchbaseCloudCluster
	}
}

func (cloudFormationStack *CloudformationStack) Claim(ctx *RegionalCloudContext, resource interface{}) {
	switch resource.(type) {
	case EC2Instance:
		ctx.Claim(resource)
		ec2Instance := resource.(EC2Instance)
		cloudFormationStack.EC2Instances[ec2Instance.ID] = ec2Instance
	}
}

func (couchbaseCloud *CouchbaseCloud) Claim(ctx *RegionalCloudContext, resource interface{}) {
	switch resource.(type) {
	case EKSCluster:
		ctx.Claim(resource)
		eksCluster := resource.(EKSCluster)
		couchbaseCloud.EKSClusters[eksCluster.Name] = eksCluster
		couchbaseCloud.Seen = true
	case CloudformationStack:
		ctx.Claim(resource)
		cloudFormationStack := resource.(CloudformationStack)
		couchbaseCloud.CloudFormationStack = &cloudFormationStack
		couchbaseCloud.Seen = true
	}
}
