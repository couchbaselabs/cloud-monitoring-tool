package monitoring

const EC2ClusterIdTagName = "DatabaseID"
const EKSClusterCloudIdTag = "CloudID"
const CloudformationCloudIdParameter = "CloudID"

type GlobalCloudContext struct {
	RegionalCloudContexts map[string]RegionalCloudContext
}

type RegionalCloudContext struct {
	Region                 string
	EBSVolumes             map[string]EBSVolume
	EC2Instances           map[string]EC2Instance
	CouchbaseCloudClusters map[string]*CouchbaseCloudCluster
	EKSClusters            map[string]EKSCluster
	CloudFormationStacks   map[string]CloudformationStack
	CouchbaseClouds        map[string]*CouchbaseCloud
}

func (ctx *GlobalCloudContext) Add(regionalCtx RegionalCloudContext) {
	ctx.RegionalCloudContexts[regionalCtx.Region] = regionalCtx
}

func (ctx *RegionalCloudContext) Claim(resource interface{}) {
	switch resource.(type) {
	case EBSVolume:
		ebsVolume := resource.(EBSVolume)
		delete(ctx.EBSVolumes, ebsVolume.ID)
	case EC2Instance:
		ec2Instance := resource.(EC2Instance)
		delete(ctx.EC2Instances, ec2Instance.ID)
	case CouchbaseCloudCluster:
		couchbaseCloudCluster := resource.(CouchbaseCloudCluster)
		delete(ctx.CouchbaseCloudClusters, couchbaseCloudCluster.ID)
	case EKSCluster:
		eksCluster := resource.(EKSCluster)
		delete(ctx.EKSClusters, eksCluster.Name)
	case CloudformationStack:
		cloudFormationStack := resource.(CloudformationStack)
		delete(ctx.CloudFormationStacks, cloudFormationStack.ID)
	case CouchbaseCloud:
		couchbaseCloud := resource.(CouchbaseCloud)
		delete(ctx.CouchbaseClouds, couchbaseCloud.ID)
	}
}

func NewGlobalCloudContext() *GlobalCloudContext {
	return &GlobalCloudContext{
		RegionalCloudContexts: make(map[string]RegionalCloudContext),
	}
}

func NewRegionalCloudContext(region string) *RegionalCloudContext {
	return &RegionalCloudContext{
		Region:                 region,
		EBSVolumes:             make(map[string]EBSVolume),
		EC2Instances:           make(map[string]EC2Instance),
		CouchbaseCloudClusters: make(map[string]*CouchbaseCloudCluster),
		EKSClusters:            make(map[string]EKSCluster),
		CloudFormationStacks:   make(map[string]CloudformationStack),
		CouchbaseClouds:        make(map[string]*CouchbaseCloud),
	}
}

func (ctx *RegionalCloudContext) GetEC2InstancesByClusterId() map[string][]EC2Instance {
	ec2Instances := map[string][]EC2Instance{}

	for _, ec2Instance := range ctx.EC2Instances {
		if clusterId, ok := ec2Instance.Tags[EC2ClusterIdTagName]; ok {
			ec2Instances[clusterId] = append(ec2Instances[clusterId], ec2Instance)
		}
	}

	return ec2Instances
}

func (ctx *RegionalCloudContext) GetEC2InstancesBySubnetId() map[string][]EC2Instance {
	ec2Instances := map[string][]EC2Instance{}

	for _, ec2Instance := range ctx.EC2Instances {
		subnetId := ec2Instance.SubnetID
		ec2Instances[subnetId] = append(ec2Instances[subnetId], ec2Instance)
	}

	return ec2Instances
}

func (ctx *RegionalCloudContext) GetCouchbaseCloudClustersByEKSName() map[string][]CouchbaseCloudCluster {
	couchbaseClusters := map[string][]CouchbaseCloudCluster{}

	for _, couchbaseCluster := range ctx.CouchbaseCloudClusters {
		if couchbaseCluster.EKSClusterName != "" {
			couchbaseClusters[couchbaseCluster.EKSClusterName] = append(couchbaseClusters[couchbaseCluster.EKSClusterName], *couchbaseCluster)
		}
	}

	return couchbaseClusters
}

func (ctx *RegionalCloudContext) GetEKSClustersByCloudId() map[string]EKSCluster {
	eksClusters := map[string]EKSCluster{}

	for _, eksCluster := range ctx.EKSClusters {
		if cloudId, ok := eksCluster.Tags[EKSClusterCloudIdTag]; ok {
			eksClusters[cloudId] = eksCluster
		}
	}

	return eksClusters
}

func (ctx *RegionalCloudContext) GetCloudformationStacksByCloudId() map[string]CloudformationStack {
	cloudformationStacks := map[string]CloudformationStack{}

	for _, cloudformationStack := range ctx.CloudFormationStacks {
		if cloudId, ok := cloudformationStack.Parameters[CloudformationCloudIdParameter]; ok {
			cloudformationStacks[cloudId] = cloudformationStack
		}
	}

	return cloudformationStacks
}

